package auctions

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/plugins"
)

func TestAuctionCopyoverParticipation(t *testing.T) {
	// Create a test auction module
	mod := &AuctionsModule{
		plug: plugins.New(`auctions`, `1.0`),
		auctionMgr: AuctionManager{
			ActiveAuction:   nil,
			maxHistoryItems: 10,
			PastAuctions:    []PastAuctionItem{},
		},
	}

	t.Run("NoActiveAuction", func(t *testing.T) {
		// Test with no active auction
		canCopy, veto := mod.CanCopyover()
		if !canCopy {
			t.Error("Should allow copyover with no active auction")
		}
		if veto.Reason != "" {
			t.Error("Should have no veto reason")
		}

		// Test state gathering with no auction
		state, err := mod.GatherState()
		if err != nil {
			t.Errorf("GatherState failed: %v", err)
		}
		if state != nil {
			t.Error("Should return nil state with no auction")
		}
	})

	t.Run("ActiveAuctionFarFromEnd", func(t *testing.T) {
		// Create an auction ending in 5 minutes
		mod.auctionMgr.ActiveAuction = &AuctionItem{
			ItemData:     items.Item{ItemId: 123},
			EndTime:      time.Now().Add(5 * time.Minute),
			MinimumBid:   100,
			SellerUserId: 1,
			SellerName:   "TestSeller",
		}

		// Should allow copyover
		canCopy, veto := mod.CanCopyover()
		if !canCopy {
			t.Error("Should allow copyover when auction is far from ending")
		}
		if veto.Type != "" {
			t.Errorf("Unexpected veto: %v", veto)
		}
	})

	t.Run("ActiveAuctionNearEnd", func(t *testing.T) {
		// Create an auction ending in 90 seconds (soft veto)
		mod.auctionMgr.ActiveAuction = &AuctionItem{
			EndTime: time.Now().Add(90 * time.Second),
		}

		canCopy, veto := mod.CanCopyover()
		if canCopy {
			t.Error("Should soft veto when auction ending soon")
		}
		if veto.Type != "soft" {
			t.Errorf("Expected soft veto, got: %s", veto.Type)
		}
		if veto.Reason == "" {
			t.Error("Should have veto reason")
		}
	})

	t.Run("ActiveAuctionVeryNearEnd", func(t *testing.T) {
		// Create an auction ending in 20 seconds (hard veto)
		mod.auctionMgr.ActiveAuction = &AuctionItem{
			EndTime: time.Now().Add(20 * time.Second),
		}

		canCopy, veto := mod.CanCopyover()
		if canCopy {
			t.Error("Should hard veto when auction ending very soon")
		}
		if veto.Type != "hard" {
			t.Errorf("Expected hard veto, got: %s", veto.Type)
		}
	})

	t.Run("StateGatherAndRestore", func(t *testing.T) {
		// Create a full auction
		testAuction := &AuctionItem{
			ItemData:          items.Item{ItemId: 456},
			SellerUserId:      1,
			SellerName:        "Seller",
			Anonymous:         true,
			EndTime:           time.Now().Add(10 * time.Minute),
			MinimumBid:        50,
			HighestBid:        75,
			HighestBidUserId:  2,
			HighestBidderName: "Bidder",
			LastUpdate:        time.Now(),
		}
		mod.auctionMgr.ActiveAuction = testAuction

		// Gather state
		state, err := mod.GatherState()
		if err != nil {
			t.Fatalf("GatherState failed: %v", err)
		}
		if state == nil {
			t.Fatal("State should not be nil with active auction")
		}

		// Type assert to verify it's the right type
		auctionState, ok := state.(AuctionCopyoverState)
		if !ok {
			t.Fatalf("State wrong type: %T", state)
		}

		// Verify state contents
		if auctionState.ItemId != 456 {
			t.Errorf("ItemId mismatch: got %d, want 456", auctionState.ItemId)
		}
		if auctionState.SellerName != "Seller" {
			t.Errorf("SellerName mismatch: got %s", auctionState.SellerName)
		}
		if !auctionState.Anonymous {
			t.Error("Anonymous flag not preserved")
		}
		if auctionState.HighestBid != 75 {
			t.Errorf("HighestBid mismatch: got %d", auctionState.HighestBid)
		}

		// Clear auction and restore
		mod.auctionMgr.ActiveAuction = nil

		err = mod.RestoreState(state)
		if err != nil {
			t.Fatalf("RestoreState failed: %v", err)
		}

		// Verify restoration
		if mod.auctionMgr.ActiveAuction == nil {
			t.Fatal("Auction not restored")
		}

		restored := mod.auctionMgr.ActiveAuction
		if restored.ItemData.ItemId != 456 {
			t.Errorf("Restored ItemId mismatch: got %d", restored.ItemData.ItemId)
		}
		if restored.SellerName != "Seller" {
			t.Errorf("Restored SellerName mismatch: got %s", restored.SellerName)
		}
		if restored.HighestBid != 75 {
			t.Errorf("Restored HighestBid mismatch: got %d", restored.HighestBid)
		}
	})

	t.Run("PrepareAndCleanup", func(t *testing.T) {
		// These should just work without errors
		if err := mod.PrepareCopyover(); err != nil {
			t.Errorf("PrepareCopyover failed: %v", err)
		}

		if err := mod.CleanupCopyover(); err != nil {
			t.Errorf("CleanupCopyover failed: %v", err)
		}
	})
}

func TestAuctionModuleRegistration(t *testing.T) {
	// This would test actual registration, but we can't easily
	// test the init() function. In practice, the auction module
	// will register itself when loaded.

	// We can at least verify the module name
	mod := &AuctionsModule{}
	if mod.ModuleName() != "auctions" {
		t.Errorf("Expected module name 'auctions', got %s", mod.ModuleName())
	}
}
