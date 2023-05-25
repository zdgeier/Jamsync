// Author: Zach Geier (zach@jamsync.dev)

package fastcdc

import (
	"io"

	"github.com/zdgeier/jamsync/gen/pb"
)

type OpType byte

type ChunkHashWriter func(ch *pb.ChunkHash) error
type OperationWriter func(op *pb.Operation) error

func (c *Chunker) CreateSignature(sw ChunkHashWriter) error {
	for {
		chunk, err := c.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		sw(&pb.ChunkHash{
			Offset: chunk.Offset,
			Length: chunk.Length,
			Hash:   chunk.Hash,
		})
	}
	return nil
}

func (c *Chunker) ApplyDelta(alignedTarget io.Writer, target io.ReadSeeker, ops chan *pb.Operation) error {
	var err error
	var n int
	var block []byte

	writeBlock := func(op *pb.Operation) error {
		_, err := target.Seek(int64(op.ChunkHash.Offset), 0)
		if err != nil {
			return err
		}
		buffer := make([]byte, int(op.ChunkHash.Length)) // TODO: reuse this buffer
		n, err = target.Read(buffer)
		if err != nil {
			if err != io.ErrUnexpectedEOF {
				return err
			}
		}
		block = buffer[:n]
		_, err = alignedTarget.Write(block)
		if err != nil {
			return err
		}
		return nil
	}

	for op := range ops {
		switch op.Type {
		case pb.Operation_OpBlock:
			err = writeBlock(op)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
		case pb.Operation_OpData:
			_, err = alignedTarget.Write(op.Chunk.Data)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Writes delta ops that are diffs from chunkhashes
func (c *Chunker) CreateDelta(chunkHashes []*pb.ChunkHash, ops OperationWriter) error {
	for i := 0; ; i++ {
		chunk, err := c.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Has valid chunk hash to compare against
		if i < len(chunkHashes) {
			chunkHash := chunkHashes[i]
			if chunkHash.Hash == chunk.Hash && chunkHash.Length == chunk.Length && chunkHash.Offset == chunk.Offset {
				ops(&pb.Operation{
					Type:      pb.Operation_OpBlock,
					ChunkHash: chunkHash,
				})
			} else {
				ops(&pb.Operation{
					Type:  pb.Operation_OpData,
					Chunk: chunk,
				})
			}
		} else {
			ops(&pb.Operation{
				Type:  pb.Operation_OpData,
				Chunk: chunk,
			})

		}
	}
	return nil
}
