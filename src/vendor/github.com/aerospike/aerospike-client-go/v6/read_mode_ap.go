/*
 * Copyright 2014-2022 Aerospike, Inc.
 *
 * Portions may be licensed to Aerospike, Inc. under one or more contributor
 * license agreements.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy of
 * the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations under
 * the License.
 */

package aerospike

import kvs "github.com/aerospike/aerospike-client-go/v6/proto/kvs"

// ReadModeAP is the read policy in AP (availability) mode namespaces.
// It indicates how duplicates should be consulted in a read operation.
// Only makes a difference during migrations and only applicable in AP mode.
type ReadModeAP int

const (
    // ReadModeAPOne indicates that a single node should be involved in the read operation.
    ReadModeAPOne ReadModeAP = iota

    // ReadModeAPAll indicates that all duplicates should be consulted in
    // the read operation.
    ReadModeAPAll
)

func (rm ReadModeAP) grpc() kvs.ReadModeAP {
    switch rm {
    case ReadModeAPOne:
        return kvs.ReadModeAP_ONE
    case ReadModeAPAll:
        return kvs.ReadModeAP_ALL
    }
    panic("UNREACHABLE")
}
