package ws

import (
	"sync"
	"testing"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Update{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestSeqManager_Init_EmptyDB(t *testing.T) {
	db := setupTestDB(t)
	sm := NewSeqManager(db)
	err := sm.Init()
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if sm.Max() != 0 {
		t.Errorf("Max() = %d, want 0 after init on empty table", sm.Max())
	}
}

func TestSeqManager_Init_WithExistingUpdates(t *testing.T) {
	db := setupTestDB(t)

	// 写入一些已有数据
	db.Create(&model.Update{ID: "1", Seq: 1, Type: "test", SessionID: "s1"})
	db.Create(&model.Update{ID: "2", Seq: 5, Type: "test", SessionID: "s1"})
	db.Create(&model.Update{ID: "3", Seq: 3, Type: "test", SessionID: "s1"})

	sm := NewSeqManager(db)
	if err := sm.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if sm.Max() != 5 {
		t.Errorf("Max() = %d, want 5", sm.Max())
	}
	// Next 应该从 6 开始
	if got := sm.Next(); got != 6 {
		t.Errorf("Next() = %d, want 6", got)
	}
}

func TestSeqManager_Next(t *testing.T) {
	sm := &SeqManager{maxSeq: 0}
	if got := sm.Next(); got != 1 {
		t.Errorf("Next() = %d, want 1", got)
	}
	if got := sm.Next(); got != 2 {
		t.Errorf("Next() = %d, want 2", got)
	}
}

func TestSeqManager_Max(t *testing.T) {
	sm := &SeqManager{maxSeq: 42}
	if got := sm.Max(); got != 42 {
		t.Errorf("Max() = %d, want 42", got)
	}
}

func TestSeqManager_Next_Concurrent(t *testing.T) {
	sm := &SeqManager{maxSeq: 0}
	const goroutines = 100
	var wg sync.WaitGroup
	results := make(chan int64, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- sm.Next()
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[int64]bool)
	for seq := range results {
		if seen[seq] {
			t.Fatalf("duplicate seq: %d", seq)
		}
		seen[seq] = true
	}
	if len(seen) != goroutines {
		t.Errorf("expected %d unique sequences, got %d", goroutines, len(seen))
	}
}
