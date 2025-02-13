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
	"encoding/base64"
	"fmt"

	"github.com/aerospike/aerospike-client-go/v6/types"
)

const (
	ctxTypeListIndex = 0x10
	ctxTypeListRank  = 0x11
	ctxTypeListValue = 0x13
	ctxTypeMapIndex  = 0x20
	ctxTypeMapRank   = 0x21
	ctxTypeMapKey    = 0x22
	ctxTypeMapValue  = 0x23
)

// CDTContext defines Nested CDT context. Identifies the location of nested list/map to apply the operation.
// for the current level.
// An array of CTX identifies location of the list/map on multiple
// levels on nesting.
type CDTContext struct {
	id    int
	value Value
}

// CDTContextToBase64 converts a []*CDTContext into a base64 encoded string.
func CDTContextToBase64(ctxl []*CDTContext) (string, Error) {
	ctx := cdtContextList(ctxl)
	sz, err := ctx.packArray(nil)
	if err != nil {
		return "", err
	}

	buf := newBuffer(sz)
	_, err = ctx.packArray(buf)
	if err != nil {
		return "", err
	}

	b64 := base64.StdEncoding.EncodeToString(buf.dataBuffer)
	return b64, nil
}

// Base64ToCDTContext converts a b64 encoded string back into a []*CDTContext.
func Base64ToCDTContext(b64 string) ([]*CDTContext, Error) {
	msg, err1 := base64.StdEncoding.DecodeString(b64)
	if err1 != nil {
		return nil, newError(types.PARSE_ERROR, err1.Error())
	}

	unpacker := newUnpacker(msg, 0, len(msg))
	list, err := unpacker.UnpackList()
	if err != nil {
		return nil, err
	}

	if len(list)%2 != 0 {
		return nil, newError(types.PARSE_ERROR, "List count must be even")
	}

	res := make([]*CDTContext, 0, len(list)/2)
	for i := 0; i < len(list); i += 2 {
		res = append(res, &CDTContext{id: list[i].(int), value: NewValue(list[i+1])})
	}

	return res, nil
}

// String implements the Stringer interface for CDTContext
func (ctx *CDTContext) String() string {
	return fmt.Sprintf("CDTContext{id: %d, value: %s}", ctx.id, ctx.value.String())
}

func (ctx *CDTContext) pack(cmd BufferEx) (int, Error) {
	size := 0
	sz, err := packAInt64(cmd, int64(ctx.id))
	size += sz
	if err != nil {
		return size, err
	}

	sz, err = ctx.value.pack(cmd)
	size += sz

	return size, err
}

// cdtContextList is used in FilterExpression API
type cdtContextList []*CDTContext

func (ctxl cdtContextList) pack(cmd BufferEx) (int, Error) {
	size := 0
	for i := range ctxl {
		sz, err := ctxl[i].pack(cmd)
		size += sz
		if err != nil {
			return size, err
		}
	}

	return size, nil
}

// used in CreateComplexIndex
func (ctxl cdtContextList) packArray(cmd BufferEx) (int, Error) {
	size, err := packArrayBegin(cmd, len(ctxl)*2)
	if err != nil {
		return size, err
	}

	for i := range ctxl {
		sz, err := ctxl[i].pack(cmd)
		size += sz
		if err != nil {
			return size, err
		}
	}

	return size, nil
}

// CtxListIndex defines Lookup list by index offset.
// If the index is negative, the resolved index starts backwards from end of list.
// If an index is out of bounds, a parameter error will be returned.
// Examples:
// 0: First item.
// 4: Fifth item.
// -1: Last item.
// -3: Third to last item.
func CtxListIndex(index int) *CDTContext {
	return &CDTContext{ctxTypeListIndex, IntegerValue(index)}
}

// CtxListIndexCreate list with given type at index offset, given an order and pad.
func CtxListIndexCreate(index int, order ListOrderType, pad bool) *CDTContext {
	return &CDTContext{ctxTypeListIndex | cdtListOrderFlag(order, pad), IntegerValue(index)}
}

// CtxListRank defines Lookup list by rank.
// 0 = smallest value
// N = Nth smallest value
// -1 = largest value
func CtxListRank(rank int) *CDTContext {
	return &CDTContext{ctxTypeListRank, IntegerValue(rank)}
}

// CtxListValue defines Lookup list by value.
func CtxListValue(key Value) *CDTContext {
	return &CDTContext{ctxTypeListValue, key}
}

// CtxMapIndex defines Lookup map by index offset.
// If the index is negative, the resolved index starts backwards from end of list.
// If an index is out of bounds, a parameter error will be returned.
// Examples:
// 0: First item.
// 4: Fifth item.
// -1: Last item.
// -3: Third to last item.
func CtxMapIndex(index int) *CDTContext {
	return &CDTContext{ctxTypeMapIndex, IntegerValue(index)}
}

// CtxMapRank defines Lookup map by rank.
// 0 = smallest value
// N = Nth smallest value
// -1 = largest value
func CtxMapRank(rank int) *CDTContext {
	return &CDTContext{ctxTypeMapRank, IntegerValue(rank)}
}

// CtxMapKey defines Lookup map by key.
func CtxMapKey(key Value) *CDTContext {
	return &CDTContext{ctxTypeMapKey, key}
}

// CtxMapKeyCreate creates map with given type at map key.
func CtxMapKeyCreate(key Value, order mapOrderType) *CDTContext {
	return &CDTContext{ctxTypeMapKey | order.flag, key}
}

// CtxMapValue defines Lookup map by value.
func CtxMapValue(key Value) *CDTContext {
	return &CDTContext{ctxTypeMapValue, key}
}
