// Copyright 2014-2022 Aerospike, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aerospike

import (
	"time"

	kvs "github.com/aerospike/aerospike-client-go/v6/proto/kvs"
)

// ScanPolicy encapsulates parameters used in scan operations.
type ScanPolicy struct {
	MultiPolicy
}

// NewScanPolicy creates a new ScanPolicy instance with default values.
// Set MaxRetries for scans on server versions >= 4.9. All other
// scans are not retried.
//
// The latest servers support retries on individual data partitions.
// This feature is useful when a cluster is migrating and partition(s)
// are missed or incomplete on the first scan attempt.
//
// If the first scan attempt misses 2 of 4096 partitions, then only
// those 2 partitions are retried in the next scan attempt from the
// last key digest received for each respective partition.  A higher
// default MaxRetries is used because it's wasteful to invalidate
// all scan results because a single partition was missed.
func NewScanPolicy() *ScanPolicy {
	mp := *NewMultiPolicy()
	mp.TotalTimeout = 0

	return &ScanPolicy{
		MultiPolicy: mp,
	}
}

func (sp *ScanPolicy) grpc() *kvs.ScanPolicy {
	TotalTimeout := uint32(sp.TotalTimeout / time.Millisecond)
	MaxRecords := uint64(sp.MaxRecords)
	RecordsPerSecond := uint32(sp.RecordsPerSecond)
	MaxConcurrentNodes := uint32(sp.MaxConcurrentNodes)
	IncludeBinData := sp.IncludeBinData
	ConcurrentNodes := MaxConcurrentNodes > 1

	return &kvs.ScanPolicy{
		Replica:            sp.ReplicaPolicy.grpc(),
		ReadModeAP:         sp.ReadModeAP.grpc(),
		ReadModeSC:         sp.ReadModeSC.grpc(),
		Compress:           sp.UseCompression,
		Expression:         sp.FilterExpression.grpc(),
		TotalTimeout:       &TotalTimeout,
		MaxRecords:         &MaxRecords,
		RecordsPerSecond:   &RecordsPerSecond,
		ConcurrentNodes:    &ConcurrentNodes,
		MaxConcurrentNodes: &MaxConcurrentNodes,
		IncludeBinData:     &IncludeBinData,
	}
}
