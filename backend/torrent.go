// Copyright 2020 The CortexTheseus Authors
// This file is part of the CortexTheseus library.
//
// The CortexTheseus library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The CortexTheseus library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the CortexTheseus library. If not, see <http://www.gnu.org/licenses/>.

package backend

import (
	"errors"
	//"bytes"
	//"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CortexFoundation/CortexTheseus/common"
	"github.com/CortexFoundation/CortexTheseus/common/mclock"
	"github.com/CortexFoundation/CortexTheseus/log"
	"github.com/anacrolix/torrent"
	//"github.com/anacrolix/torrent/metainfo"
	//"github.com/anacrolix/torrent/storage"
	"github.com/CortexFoundation/torrentfs/params"
)

const (
	torrentPending = iota + 1
	torrentPaused
	torrentRunning
	torrentSeeding
	//torrentSleeping
)

type Torrent struct {
	*torrent.Torrent
	//maxEstablishedConns int
	//minEstablishedConns int
	//currentConns        int
	bytesRequested int64
	//bytesLimitation int64
	//bytesCompleted int64
	//bytesMissing        int64
	status   int
	infohash string
	filepath string
	cited    atomic.Int32
	//weight     int
	//loop       int
	maxPieces int
	//isBoosting bool
	//fast  bool
	start mclock.AbsTime
	//ch    chan bool
	wg sync.WaitGroup

	lock sync.RWMutex

	closeAll chan any

	taskCh chan task

	slot int

	once sync.Once
}

type task struct {
	start int
	end   int
}

func NewTorrent(t *torrent.Torrent, requested int64, ih string, path string, slot int) *Torrent {
	tor := Torrent{
		Torrent:        t,
		bytesRequested: requested,
		status:         torrentPending,
		infohash:       ih,
		filepath:       path,
		start:          mclock.Now(),
		taskCh:         make(chan task, 1),
		closeAll:       make(chan any),
		slot:           slot,
	}

	//tor.wg.Add(1)
	//go tor.listen()

	return &tor
}

func (t *Torrent) QuotaFull() bool {
	//t.RLock()
	//defer t.RUnlock()

	return t.Info() != nil && t.bytesRequested >= t.Length()
}

func (t *Torrent) Birth() mclock.AbsTime {
	return t.start
}

func (t *Torrent) Lock() {
	t.lock.Lock()
}

func (t *Torrent) Unlock() {
	t.lock.Unlock()
}

func (t *Torrent) RLock() {
	t.lock.RLock()
}

func (t *Torrent) RUnlock() {
	t.lock.RUnlock()
}

/*func (t *Torrent) BytesLeft() int64 {
	if t.bytesRequested < t.bytesCompleted {
		return 0
	}
	return t.bytesRequested - t.bytesCompleted
}*/

func (t *Torrent) InfoHash() string {
	return t.infohash
}

func (t *Torrent) Status() int {
	return t.status
}

func (t *Torrent) Cited() int32 {
	return t.cited.Load()
}

func (t *Torrent) CitedInc() {
	t.cited.Add(1)
}

func (t *Torrent) CitedDec() {
	t.cited.Add(-1)
}

func (t *Torrent) BytesRequested() int64 {
	//t.RLock()
	//defer t.RUnlock()

	return t.bytesRequested
}

func (t *Torrent) SetBytesRequested(bytesRequested int64) {
	t.Lock()
	defer t.Unlock()
	t.bytesRequested = bytesRequested
}

func (t *Torrent) Ready() bool {
	t.RLock()
	defer t.RUnlock()

	if _, ok := params.BadFiles[t.InfoHash()]; ok {
		return false
	}

	ret := t.IsSeeding()
	if !ret {
		log.Debug("Not ready", "ih", t.InfoHash(), "status", t.status, "seed", t.Torrent.Seeding(), "seeding", torrentSeeding)
	}

	return ret
}

func (t *Torrent) WriteTorrent() error {
	t.Lock()
	defer t.Unlock()
	if _, err := os.Stat(filepath.Join(t.filepath, TORRENT)); err == nil {
		//t.Pause()
		return nil
	}

	if f, err := os.OpenFile(filepath.Join(t.filepath, TORRENT), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777); err == nil {
		defer f.Close()
		log.Debug("Write seed file", "path", t.filepath)
		if err := t.Metainfo().Write(f); err != nil {
			log.Warn("Write seed error", "err", err)
			return err
		}
	} else {
		log.Warn("Create Path error", "err", err)
		return err
	}

	return nil
}

//func (t *Torrent) BoostOff() {
//t.isBoosting = false
//}

func (t *Torrent) Seed() bool {
	//t.lock.Lock()
	//defer t.lock.Unlock()

	if t.Torrent.Info() == nil {
		log.Debug("Torrent info is nil", "ih", t.InfoHash())
		return false
	}
	if t.status == torrentSeeding {
		log.Debug("Torrent status is", "status", t.status, "ih", t.InfoHash())
		return true
	}
	//if t.currentConns <= t.minEstablishedConns {
	//t.setCurrentConns(t.maxEstablishedConns)
	//t.Torrent.SetMaxEstablishedConns(t.currentConns)
	//}
	if t.Torrent.Seeding() {
		t.Lock()
		t.status = torrentSeeding
		t.Unlock()

		elapsed := time.Duration(mclock.Now()) - time.Duration(t.start)
		//if active, ok := params.GoodFiles[t.InfoHash()]; !ok {
		//	log.Info("New active nas found", "ih", t.InfoHash(), "ok", ok, "active", active, "size", common.StorageSize(t.BytesCompleted()), "files", len(t.Files()), "pieces", t.Torrent.NumPieces(), "seg", len(t.Torrent.PieceStateRuns()), "peers", t.currentConns, "status", t.status, "elapsed", common.PrettyDuration(elapsed))
		//} else {
		log.Info("Imported new nas segment", "ih", t.InfoHash(), "size", common.StorageSize(t.Torrent.BytesCompleted()), "files", len(t.Files()), "pieces", t.Torrent.NumPieces(), "seg", len(t.Torrent.PieceStateRuns()), "status", t.status, "elapsed", common.PrettyDuration(elapsed), "speed", common.StorageSize(float64(t.Torrent.BytesCompleted()*1000*1000*1000)/float64(elapsed)).String()+"/s")
		//}
		return true
	}

	return false
}

func (t *Torrent) IsSeeding() bool {
	return t.status == torrentSeeding && t.Torrent.Seeding()
}

func (t *Torrent) Pause() {
	//if t.currentConns > t.minEstablishedConns {
	//t.setCurrentConns(t.minEstablishedConns)
	//t.Torrent.SetMaxEstablishedConns(t.minEstablishedConns)
	//}
	if t.status != torrentPaused {
		t.status = torrentPaused
		t.maxPieces = 0 //t.minEstablishedConns
		t.Torrent.CancelPieces(0, t.Torrent.NumPieces())
	}
}

func (t *Torrent) Paused() bool {
	return t.status == torrentPaused
}

func (t *Torrent) Leech() error {
	// Make sure the torrent info exists
	if t.Torrent.Info() == nil {
		return errors.New("info is nil")
	}

	t.Lock()
	defer t.Unlock()

	if t.status != torrentRunning {
		return errors.New("torrent is not running")
	}

	limitPieces := int((t.bytesRequested*int64(t.Torrent.NumPieces()) + t.Length() - 1) / t.Length())
	if limitPieces > t.Torrent.NumPieces() {
		limitPieces = t.Torrent.NumPieces()
	}

	//if limitPieces <= t.maxPieces && t.status == torrentRunning {
	//	return
	//}

	//if t.fast {
	//if t.currentConns <= t.minEstablishedConns {
	//t.setCurrentConns(t.maxEstablishedConns)
	//t.Torrent.SetMaxEstablishedConns(t.currentConns)
	//}
	//} else {
	//	if t.currentConns > t.minEstablishedConns {
	//		t.setCurrentConns(t.minEstablishedConns)
	//		t.Torrent.SetMaxEstablishedConns(t.currentConns)
	//	}
	//}
	if limitPieces > t.maxPieces {
		//t.maxPieces = limitPieces
		if err := t.download(limitPieces); err == nil {
			t.maxPieces = limitPieces
		} else {
			return err
		}
	}

	return nil
}

// Find out the start and end
func (t *Torrent) download(p int) error {
	var s, e int
	s = (t.Torrent.NumPieces() * t.slot) / bucket
	s = s - p/2
	if s < 0 {
		s = 0
	}

	if t.Torrent.NumPieces() < s+p {
		s = t.Torrent.NumPieces() - p
	}

	e = s + p

	t.taskCh <- task{s, e}
	log.Info(ScaleBar(s, e, t.Torrent.NumPieces()), "ih", t.InfoHash(), "slot", t.slot, "s", s, "e", e, "p", p, "total", t.Torrent.NumPieces())
	return nil
}

func (t *Torrent) run() bool {
	t.Lock()
	defer t.Unlock()

	if t.Info() != nil {
		t.status = torrentRunning
	} else {
		log.Warn("Task listener not ready", "ih", t.InfoHash())
		return false
	}

	return true
}

func (t *Torrent) listen() {
	defer t.wg.Done()

	if !t.run() {
		return
	}

	log.Info("Task listener started", "ih", t.InfoHash())

	for {
		select {
		case task := <-t.taskCh:
			t.Torrent.DownloadPieces(task.start, task.end)
		case <-t.closeAll:
			log.Info("Task listener stopped", "ih", t.InfoHash())
			return
		}
	}
}

func (t *Torrent) Running() bool {
	return t.status == torrentRunning
}

func (t *Torrent) Pending() bool {
	return t.status == torrentPending
}

func (t *Torrent) Start() error {
	t.once.Do(func() {
		t.wg.Add(1)
		go t.listen()
	})
	return nil
}

func (t *Torrent) Stop() {
	t.Lock()
	defer t.Unlock()

	defer t.Torrent.Drop()

	close(t.closeAll)

	t.wg.Wait()

	log.Info(ProgressBar(t.BytesCompleted(), t.Torrent.Length(), ""), "ih", t.InfoHash(), "total", common.StorageSize(t.Torrent.Length()), "req", common.StorageSize(t.BytesRequested()), "finish", common.StorageSize(t.Torrent.BytesCompleted()), "status", t.Status(), "cited", t.Cited())
}
