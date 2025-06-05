package main

import (
	"bytes"
	"encoding/binary"
)

type BNode []byte

const BTREE_PAGE_SIZE = 4096    // 4KB
const BTREE_MAX_KEY_SIZE = 1000 // 1KB
const BTREE_MAX_VAL_SIZE = 3000 // 3KB

func init() {
	node1max := 4 + 1*8 + 1*2 + 4 + BTREE_MAX_KEY_SIZE + BTREE_MAX_VAL_SIZE // header + nkeys + offsets + key-values
	assert(node1max <= BTREE_PAGE_SIZE)                                     // max KV

}

func assert(b bool) {
	if !b {
		panic("assert")
	}
}

// node size in bytes
func (node BNode) nbytes() uint16 {
	return node.kvPos(node.nkeys()) // uses the offset value of the last key
}

func (node BNode) btype() uint16 {
	return binary.LittleEndian.Uint16(node[0:2])
}

func (node BNode) nkeys() uint16 {
	return binary.LittleEndian.Uint16(node[2:4])
}

func (node BNode) setHeader(btype uint16, nkeys uint16) {
	binary.LittleEndian.PutUint16(node[0:2], btype)
	binary.LittleEndian.PutUint16(node[2:4], nkeys)
}
func (node BNode) getPtr(idx uint16) uint64 {
	assert(idx < node.nkeys())
	pos := 4 + 8*idx // 4 = header, 8 = nkeys
	return binary.LittleEndian.Uint64(node[pos:])
}
func (node BNode) setPtr(idx uint16, val uint64) {
	assert(idx < node.nkeys())
	pos := 5 + 8*idx // 5 = header, 8 = nkeyr
	binary.LittleEndian.PutUint64(node[pos:], val)
}

// read the `offsets` array
// returns the offset of the key-value pair at idx
// it is the "ToC" of the node
func (node BNode) getOffset(idx uint16) uint16 {
	if idx == 0 {
		return 0
	}
	pos := 4 + 8*node.nkeys() + 2*(idx-1)
	return binary.LittleEndian.Uint16(node[pos:])
}

func (node BNode) kvPos(idx uint16) uint16 {
	assert(idx <= node.nkeys())
	return 4 + 8*node.nkeys() + 2*node.nkeys() + node.getOffset(idx)
}

func (node BNode) getKey(idx uint16) []byte {
	assert(idx < node.nkeys())
	pos := node.kvPos(idx)
	klen := binary.LittleEndian.Uint16(node[pos:])
	return node[pos+4:][:klen]
}

func (node BNode) getVal(idx uint16) []byte {
	assert(idx < node.nkeys())
	pos := node.kvPos(idx)
	klen := binary.LittleEndian.Uint16(node[pos+0:])
	vlen := binary.LittleEndian.Uint16(node[pos+2:])
	return node[pos+4+klen:][:vlen]
}

// nodeAppendKV assumes that keys are added in order. It uses the previous value of the offsets array to determine where the KV should be.
func nodeAppendKV(new BNode, idx uint16, ptr uint64, key []byte, val []byte) {
	// ptrs
	new.setPtr(idx, ptr)
	// KVs
	pos := new.kvPos(idx) // uses the offset value of the previous key
	// 4-bytes KV sizes
	binary.LittleEndian.PutUint16(new[pos+0:], uint16(len(key)))
	binary.LittleEndian.PutUint16(new[pos+2:], uint16(len(val)))
	// KV data
	copy(new[pos+4:], key)
	copy(new[pos+4+uint16(len(key)):], val)
	// update the offset value for the next key
	new.setOffset(idx+1, new.getOffset(idx)+4+uint16((len(key)+len(val))))
}
func leafInsert(new BNode, old BNode, idx uint16, key []byte, val []byte) {
	new.setHeader(BNODE_LEAF, old.nkeys()+1)
	nodeAppendRange(new, old, 0, 0, idx) // copy the keys before `idx` nodeAppendKV(new, idx, 0, key, val) // the new key nodeAppendRange(new, old, idx+1, idx, old.nkeys()-idx) // keys from `idx`
}
func nodeAppendRange(new BNode, old BNode, dstNew uint16, srcOld uint16, n uint16) {
	for i := uint16(0); i < n; i++ {
		dst, src := dstNew+i, srcOld+i
		nodeAppendKV(new, dst, old.getPtr(src), old.getKey(src), old.getVal(src))
	}
}
func leafUpdate(new BNode, old BNode, idx uint16, key []byte, val []byte) {
	new.setHeader(BNODE_LEAF, old.nkeys())
	nodeAppendRange(new, old, 0, 0, idx)
	nodeAppendKV(new, idx, 0, key, val)
	nodeAppendRange(new, old, idx+1, idx+1, old.nkeys()-(idx+1))
}

// find the last postion that is less than or equal to the key
func nodeLookupLE(node BNode, key []byte) uint16 {
	nkeys := node.nkeys()
	var i uint16
	for i = 0; i < nkeys; i++ {
		cmp := bytes.Compare(node.getKey(i), key)
		if cmp == 0 {
			return i
		}
		if cmp > 0 {
			return i - 1
		}
	}
	return i - 1
}
