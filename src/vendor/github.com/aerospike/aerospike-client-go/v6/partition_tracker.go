// Copyright 2014-2022 Aerospike, Inc.
//
// Portions may be licensed to Aerospike, Inc. under one or more contributor
// license agreements WHICH ARE COMPATIBLE WITH THE APACHE LICENSE, VERSION 2.0.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package aerospike

import (
	"fmt"
	"strings"
	"time"

	"github.com/aerospike/aerospike-client-go/v6/types"
)

type partitionTracker struct {
	partitions          []*partitionStatus
	partitionsCapacity  int
	partitionBegin      int
	nodeCapacity        int
	nodeFilter          *Node
	partitionFilter     *PartitionFilter
	nodePartitionsList  []*nodePartitions
	maxRecords          int64
	sleepBetweenRetries time.Duration
	socketTimeout       time.Duration
	totalTimeout        time.Duration
	iteration           int //= 1
	deadline            time.Time
}

func newPartitionTrackerForNodes(policy *MultiPolicy, nodes []*Node) *partitionTracker {
	// Create initial partition capacity for each node as average + 25%.
	ppn := _PARTITIONS / len(nodes)
	ppn += ppn / 4

	pt := partitionTracker{
		partitionBegin:     0,
		nodeCapacity:       len(nodes),
		nodeFilter:         nil,
		partitionsCapacity: ppn,
		maxRecords:         policy.MaxRecords,
		iteration:          1,
	}

	pt.partitions = pt.initPartitions(policy, _PARTITIONS, nil)
	pt.initTimeout(policy)
	return &pt
}

func newPartitionTrackerForNode(policy *MultiPolicy, nodeFilter *Node) *partitionTracker {
	pt := partitionTracker{
		partitionBegin:     0,
		nodeCapacity:       1,
		nodeFilter:         nodeFilter,
		partitionsCapacity: _PARTITIONS,
		maxRecords:         policy.MaxRecords,
		iteration:          1,
	}

	pt.partitions = pt.initPartitions(policy, _PARTITIONS, nil)
	pt.initTimeout(policy)
	return &pt
}

func newPartitionTracker(policy *MultiPolicy, filter *PartitionFilter, nodes []*Node) *partitionTracker {
	// Validate here instead of initial PartitionFilter constructor because total number of
	// cluster partitions may change on the server and PartitionFilter will never have access
	// to Cluster instance.  Use fixed number of partitions for now.
	if !(filter.begin >= 0 && filter.begin < _PARTITIONS) {
		panic(newError(types.PARAMETER_ERROR, fmt.Sprintf("Invalid partition begin %d . Valid range: 0-%d", filter.begin,
			(_PARTITIONS-1))))
	}

	if filter.count <= 0 {
		panic(newError(types.PARAMETER_ERROR, fmt.Sprintf("Invalid partition count %d", filter.count)))
	}

	if filter.begin+filter.count > _PARTITIONS {
		panic(newError(types.PARAMETER_ERROR, fmt.Sprintf("Invalid partition range (%d,%d)", filter.begin, filter.begin+filter.count)))
	}

	pt := &partitionTracker{
		partitionBegin:     filter.begin,
		nodeCapacity:       len(nodes),
		nodeFilter:         nil,
		partitionsCapacity: filter.count,
		maxRecords:         policy.MaxRecords,
		iteration:          1,
	}

	if len(filter.partitions) == 0 {
		filter.partitions = pt.initPartitions(policy, filter.count, filter.digest)
	} else {
		// Retry all partitions when maxRecords not specified.
		if policy.MaxRecords <= 0 {
			for _, ps := range filter.partitions {
				ps.Retry = true
			}
		}
	}

	pt.partitions = filter.partitions
	pt.partitionFilter = filter
	pt.initTimeout(policy)
	return pt
}

func (pt *partitionTracker) initTimeout(policy *MultiPolicy) {
	pt.sleepBetweenRetries = policy.SleepBetweenRetries
	pt.socketTimeout = policy.SocketTimeout
	pt.totalTimeout = policy.TotalTimeout
	if pt.totalTimeout > 0 {
		pt.deadline = time.Now().Add(pt.totalTimeout)
		if pt.socketTimeout == 0 || pt.socketTimeout > pt.totalTimeout {
			pt.socketTimeout = pt.totalTimeout
		}
	}
}

func (pt *partitionTracker) initPartitions(policy *MultiPolicy, partitionCount int, digest []byte) []*partitionStatus {
	partsAll := make([]*partitionStatus, partitionCount)

	for i := 0; i < partitionCount; i++ {
		partsAll[i] = newPartitionStatus(pt.partitionBegin + i)
	}

	if digest != nil {
		partsAll[0].Digest = digest
	}

	pt.sleepBetweenRetries = policy.SleepBetweenRetries
	pt.socketTimeout = policy.SocketTimeout
	pt.totalTimeout = policy.TotalTimeout

	if pt.totalTimeout > 0 {
		pt.deadline = time.Now().Add(pt.totalTimeout)

		if pt.socketTimeout == 0 || pt.socketTimeout > pt.totalTimeout {
			pt.socketTimeout = pt.totalTimeout
		}
	}

	return partsAll
}

func (pt *partitionTracker) SetSleepBetweenRetries(sleepBetweenRetries time.Duration) {
	pt.sleepBetweenRetries = sleepBetweenRetries
}

func (pt *partitionTracker) assignPartitionsToNodes(cluster *Cluster, namespace string) ([]*nodePartitions, Error) {
	list := make([]*nodePartitions, 0, pt.nodeCapacity)

	pMap := cluster.getPartitions()
	partitions := pMap[namespace]

	if partitions == nil {
		return nil, newError(types.INVALID_NAMESPACE, fmt.Sprintf("Invalid Partition Map for namespace `%s` in Partition Scan", namespace))
	}

	master := partitions.Replicas[0]

	for _, part := range pt.partitions {
		if part != nil && part.Retry {
			node := master[part.Id]

			if node == nil {
				return nil, newError(types.INVALID_NAMESPACE, fmt.Sprintf("Invalid Partition Id %d for namespace `%s` in Partition Scan", part.Id, namespace))
			}

			if pt.iteration == 1 {
				part.replicaIndex = 0
			} else {
				// If the partition was unavailable in the previous iteration, retry on
				// a different replica.
				if part.unavailable && part.node == node {
					part.replicaIndex++

					if part.replicaIndex >= len(partitions.Replicas) {
						part.replicaIndex = 0
					}

					replica := partitions.Replicas[part.replicaIndex][part.Id]

					if replica != nil {
						node = replica
					}
				}
			}

			part.node = node
			part.unavailable = false
			part.Retry = false

			// Use node name to check for single node equality because
			// partition map may be in transitional state between
			// the old and new node with the same name.
			if pt.nodeFilter != nil && pt.nodeFilter.GetName() != node.GetName() {
				continue
			}

			np := pt.findNode(list, node)

			if np == nil {
				// If the partition map is in a transitional state, multiple
				// nodePartitions instances (each with different partitions)
				// may be created for a single node.
				np = newNodePartitions(node, pt.partitionsCapacity)
				list = append(list, np)
			}
			np.addPartition(part)
		}
	}

	if pt.maxRecords > 0 {
		// Distribute maxRecords across nodes.
		nodeSize := len(list)

		if pt.maxRecords < int64(nodeSize) {
			// Only include nodes that have at least 1 record requested.
			nodeSize = int(pt.maxRecords)
			list = list[:nodeSize]
		}

		max := int64(0)
		if nodeSize > 0 {
			max = pt.maxRecords / int64(nodeSize)
		}
		rem := int(pt.maxRecords - (max * int64(nodeSize)))

		for i, np := range list[:nodeSize] {
			if i < rem {
				np.recordMax = max + 1
			} else {
				np.recordMax = max
			}
		}
	}

	pt.nodePartitionsList = list
	return list, nil
}

func (pt *partitionTracker) findNode(list []*nodePartitions, node *Node) *nodePartitions {
	for _, nodePartition := range list {
		// Use pointer equality for performance.
		if nodePartition.node == node {
			return nodePartition
		}
	}
	return nil
}

func (pt *partitionTracker) partitionUnavailable(nodePartitions *nodePartitions, partitionId int) {
	ps := pt.partitions[partitionId-pt.partitionBegin]
	ps.unavailable = true
	ps.Retry = true
	nodePartitions.partsUnavailable++
}

func (pt *partitionTracker) setDigest(nodePartitions *nodePartitions, key *Key) {
	partitionId := key.PartitionId()
	pt.partitions[partitionId-pt.partitionBegin].Digest = key.Digest()

	// nodePartitions is nil in Proxy client
	if nodePartitions != nil {
		nodePartitions.recordCount++
	}
}

func (pt *partitionTracker) setLast(nodePartitions *nodePartitions, key *Key, bval int64) {
	partitionId := key.PartitionId()
	if partitionId-pt.partitionBegin < 0 {
		panic(fmt.Sprintf("key.partitionId: %d, partitionBegin: %d", partitionId, pt.partitionBegin))
	}
	ps := pt.partitions[partitionId-pt.partitionBegin]
	ps.Digest = key.digest[:]
	ps.BVal = bval

	// nodePartitions is nil in Proxy client
	if nodePartitions != nil {
		nodePartitions.recordCount++
	}
}

func (pt *partitionTracker) isComplete(cluster *Cluster, policy *BasePolicy) (bool, Error) {
	recordCount := int64(0)
	partsUnavailable := 0

	for _, np := range pt.nodePartitionsList {
		recordCount += np.recordCount
		partsUnavailable += np.partsUnavailable
	}

	if partsUnavailable == 0 {
		if pt.maxRecords <= 0 {
			if pt.partitionFilter != nil {
				pt.partitionFilter.done = true
			}
		} else {
			// Cluster will be nil for the Proxy client
			if cluster == nil || cluster.supportsPartitionQuery.Get() {
				// Server version >= 6.0 will return all records for each node up to
				// that node's max. If node's record count reached max, there still
				// may be records available for that node.
				done := true

				for _, np := range pt.nodePartitionsList {
					if np.recordCount >= np.recordMax {
						pt.markRetry(np)
						done = false
					}
				}

				if pt.partitionFilter != nil {
					pt.partitionFilter.done = done
				}
			} else {
				// Servers version < 6.0 can return less records than max and still
				// have more records for each node, so the node is only done if no
				// records were retrieved for that node.
				for _, np := range pt.nodePartitionsList {
					if np.recordCount > 0 {
						pt.markRetry(np)
					}
				}

				if pt.partitionFilter != nil {
					pt.partitionFilter.done = (recordCount == 0)
				}
			}
		}
		return true, nil
	}

	if pt.maxRecords > 0 && recordCount >= pt.maxRecords {
		return true, nil
	}

	// Check if limits have been reached.
	if pt.iteration > policy.MaxRetries {
		return false, newError(types.MAX_RETRIES_EXCEEDED, fmt.Sprintf("Max retries exceeded: %d", policy.MaxRetries))
	}

	if policy.TotalTimeout > 0 {
		// Check for total timeout.
		remaining := time.Until(pt.deadline) - pt.sleepBetweenRetries

		if remaining <= 0 {
			return false, ErrTimeout.err()
		}

		if remaining < pt.totalTimeout {
			pt.totalTimeout = remaining

			if pt.socketTimeout > pt.totalTimeout {
				pt.socketTimeout = pt.totalTimeout
			}
		}
	}

	// Prepare for next iteration.
	if pt.maxRecords > 0 {
		pt.maxRecords -= recordCount
	}
	pt.iteration++
	return false, nil
}

func (pt *partitionTracker) shouldRetry(nodePartitions *nodePartitions, e Error) bool {
	res := e.Matches(types.TIMEOUT,
		types.NETWORK_ERROR,
		types.SERVER_NOT_AVAILABLE,
		types.INDEX_NOTFOUND)
	if res {
		pt.markRetry(nodePartitions)
		nodePartitions.partsUnavailable = len(nodePartitions.partsFull) + len(nodePartitions.partsPartial)
	}
	return res
}

func (pt *partitionTracker) markRetry(nodePartitions *nodePartitions) {
	for _, ps := range nodePartitions.partsFull {
		ps.Retry = true
	}

	for _, ps := range nodePartitions.partsPartial {
		ps.Retry = true
	}
}

func (pt *partitionTracker) String() string {
	var sb strings.Builder
	for i, ps := range pt.partitions {
		sb.WriteString(ps.String())
		if (i+1)%16 == 0 {
			sb.WriteString("\n")
		} else {
			sb.WriteString("\t")
		}
	}
	return sb.String()
}

type nodePartitions struct {
	node             *Node
	partsFull        []*partitionStatus
	partsPartial     []*partitionStatus
	recordCount      int64
	recordMax        int64
	partsUnavailable int
}

func newNodePartitions(node *Node, capacity int) *nodePartitions {
	return &nodePartitions{
		node:         node,
		partsFull:    make([]*partitionStatus, 0, capacity),
		partsPartial: make([]*partitionStatus, 0, capacity),
	}
}

func (np *nodePartitions) String() string {
	return fmt.Sprintf("Node %s: full: %d, partial: %d", np.node.String(), len(np.partsFull), len(np.partsPartial))
}

func (np *nodePartitions) addPartition(part *partitionStatus) {
	if part.Digest == nil {
		np.partsFull = append(np.partsFull, part)
	} else {
		np.partsPartial = append(np.partsPartial, part)
	}
}
