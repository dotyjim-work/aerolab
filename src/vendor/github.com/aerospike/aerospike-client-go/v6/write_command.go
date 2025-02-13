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
	"math/rand"

	kvs "github.com/aerospike/aerospike-client-go/v6/proto/kvs"
	"github.com/aerospike/aerospike-client-go/v6/types"

	Buffer "github.com/aerospike/aerospike-client-go/v6/utils/buffer"
)

// guarantee writeCommand implements command interface
var _ command = &writeCommand{}

type writeCommand struct {
	singleCommand

	policy    *WritePolicy
	bins      []*Bin
	binMap    BinMap
	operation OperationType
}

func newWriteCommand(cluster *Cluster,
	policy *WritePolicy,
	key *Key,
	bins []*Bin,
	binMap BinMap,
	operation OperationType) (writeCommand, Error) {

	var partition *Partition
	var err Error
	if cluster != nil {
		partition, err = PartitionForWrite(cluster, &policy.BasePolicy, key)
		if err != nil {
			return writeCommand{}, err
		}
	}

	newWriteCmd := writeCommand{
		singleCommand: newSingleCommand(cluster, key, partition),
		policy:        policy,
		bins:          bins,
		binMap:        binMap,
		operation:     operation,
	}

	return newWriteCmd, nil
}

func (cmd *writeCommand) getPolicy(ifc command) Policy {
	return cmd.policy
}

func (cmd *writeCommand) writeBuffer(ifc command) Error {
	return cmd.setWrite(cmd.policy, cmd.operation, cmd.key, cmd.bins, cmd.binMap)
}

func (cmd *writeCommand) getNode(ifc command) (*Node, Error) {
	return cmd.partition.GetNodeWrite(cmd.cluster)
}

func (cmd *writeCommand) prepareRetry(ifc command, isTimeout bool) bool {
	cmd.partition.PrepareRetryWrite(isTimeout)
	return true
}

func (cmd *writeCommand) parseResult(ifc command, conn *Connection) Error {
	// Read header.
	if _, err := conn.Read(cmd.dataBuffer, int(_MSG_TOTAL_HEADER_SIZE)); err != nil {
		return err
	}

	header := Buffer.BytesToInt64(cmd.dataBuffer, 0)

	// Validate header to make sure we are at the beginning of a message
	if err := cmd.validateHeader(header); err != nil {
		return err
	}

	resultCode := cmd.dataBuffer[13] & 0xFF

	if resultCode != 0 {
		if resultCode == byte(types.KEY_NOT_FOUND_ERROR) {
			return ErrKeyNotFound.err()
		} else if types.ResultCode(resultCode) == types.FILTERED_OUT {
			return ErrFilteredOut.err()
		}

		return newCustomNodeError(cmd.node, types.ResultCode(resultCode))
	}
	return cmd.emptySocket(conn)
}

func (cmd *writeCommand) isRead() bool {
	return false
}

func (cmd *writeCommand) Execute() Error {
	return cmd.execute(cmd)
}

func (cmd *writeCommand) ExecuteGRPC(clnt *ProxyClient) Error {
	cmd.dataBuffer = bufPool.Get().([]byte)
	defer cmd.grpcPutBufferBack()

	err := cmd.prepareBuffer(cmd, cmd.policy.deadline())
	if err != nil {
		return err
	}

	req := kvs.AerospikeRequestPayload{
		Id:          rand.Uint32(),
		Iteration:   1,
		Payload:     cmd.dataBuffer[:cmd.dataOffset],
		WritePolicy: cmd.policy.grpc(),
	}

	conn, err := clnt.grpcConn()
	if err != nil {
		return err
	}

	client := kvs.NewKVSClient(conn)

	ctx := cmd.policy.grpcDeadlineContext()

	res, gerr := client.Write(ctx, &req)
	if gerr != nil {
		return newGrpcError(gerr, gerr.Error())
	}

	cmd.commandWasSent = true

	defer clnt.returnGrpcConnToPool(conn)

	if res.Status != 0 {
		return newGrpcStatusError(res)
	}

	cmd.conn = newGrpcFakeConnection(res.Payload, nil)
	err = cmd.parseResult(cmd, cmd.conn)
	if err != nil {
		return err
	}

	return nil
}
