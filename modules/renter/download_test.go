package renter

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/NebulousLabs/Sia/crypto"
)

// a testFetcher simulates a host. It implements the fetcher interface.
type testFetcher struct {
	sectors   map[crypto.Hash][]byte
	pieceMap  map[uint64][]pieceData
	pieceSize uint64

	nAttempt int // total number of download attempts
	nFetch   int // number of successful download attempts

	// used to simulate real-world conditions
	delay    time.Duration // transfers will take this long
	failRate int           // transfers will randomly fail with probability 1/failRate
}

func (f *testFetcher) pieces(chunkIndex uint64) []pieceData {
	return f.pieceMap[chunkIndex]
}

func (f *testFetcher) fetch(p pieceData) ([]byte, error) {
	f.nAttempt++
	time.Sleep(f.delay)
	// randomly fail
	if n, _ := crypto.RandIntn(f.failRate); n == 0 {
		return nil, io.EOF
	}
	f.nFetch++
	return f.sectors[p.MerkleRoot], nil
}

// TestErasureDownload tests parallel downloading of erasure-coded data. It
// mocks the fetcher interface in order to directly test the downloading
// algorithm.
func TestErasureDownload(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// generate data
	const dataSize = 777
	data, err := crypto.RandBytes(dataSize)
	if err != nil {
		t.Fatal(err)
	}

	// create Reed-Solomon encoder
	rsc, err := NewRSCode(2, 10)
	if err != nil {
		t.Fatal(err)
	}

	// create hosts
	const pieceSize = 10
	hosts := make([]fetcher, rsc.NumPieces())
	for i := range hosts {
		hosts[i] = &testFetcher{
			sectors:   make(map[crypto.Hash][]byte),
			pieceMap:  make(map[uint64][]pieceData),
			pieceSize: pieceSize,

			delay:    time.Millisecond,
			failRate: 5, // 20% failure rate
		}
	}
	// make one host really slow
	hosts[0].(*testFetcher).delay = 100 * time.Millisecond
	// make one host always fail
	hosts[1].(*testFetcher).failRate = 1

	// upload data to hosts
	r := bytes.NewReader(data) // makes chunking easier
	chunk := make([]byte, pieceSize*rsc.MinPieces())
	var i uint64
	for i = uint64(0); ; i++ {
		_, err := io.ReadFull(r, chunk)
		if err == io.EOF {
			break
		} else if err != nil && err != io.ErrUnexpectedEOF {
			t.Fatal(err)
		}
		pieces, err := rsc.Encode(chunk)
		if err != nil {
			t.Fatal(err)
		}
		for j, p := range pieces {
			root := crypto.MerkleRoot(p)
			host := hosts[j%len(hosts)].(*testFetcher) // distribute evenly
			host.pieceMap[i] = append(host.pieceMap[i], pieceData{
				Chunk:      uint64(i),
				Piece:      uint64(j),
				MerkleRoot: root,
			})
			host.sectors[root] = p
		}
	}

	// check hosts (not strictly necessary)
	err = checkHosts(hosts, rsc.MinPieces(), i)
	if err != nil {
		t.Fatal(err)
	}

	// download data
	d := newFile("foo", rsc, pieceSize, dataSize).newDownload(hosts, "")
	buf := new(bytes.Buffer)
	err = d.run(buf)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(buf.Bytes(), data) {
		t.Fatal("recovered data does not match original")
	}

	/*
		// These metrics can be used to assess the efficiency of the download
		// algorithm.

		totFetch := 0
		for i, h := range hosts {
			h := h.(*testHost)
			t.Logf("Host %2d:  Fetched: %v/%v", i, h.nFetch, h.nAttempt)
			totFetch += h.nAttempt
		}
		t.Log("Optimal fetches:", i*uint64(rsc.MinPieces()))
		t.Log("Total fetches:  ", totFetch)
	*/
}
