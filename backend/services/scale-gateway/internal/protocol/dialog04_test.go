package protocol

import "testing"

func TestDialog04EncodeSetPrice(t *testing.T) {
	codec := Dialog04Codec{}
	frame, err := codec.EncodeSetPrice(1499)
	if err != nil {
		t.Fatalf("EncodeSetPrice(1499) returned error: %v", err)
	}
	want := BuildFrame([]byte("0001499"))
	if string(frame) != string(want) {
		t.Fatalf("EncodeSetPrice(1499) = % X, want % X", frame, want)
	}
}

func TestDialog04EncodeSetPriceRejectsOutOfRange(t *testing.T) {
	codec := Dialog04Codec{}
	if _, err := codec.EncodeSetPrice(10000000); err == nil {
		t.Fatal("expected error for a price that does not fit in 7 digits, got nil")
	}
	if _, err := codec.EncodeSetPrice(-1); err == nil {
		t.Fatal("expected error for a negative price, got nil")
	}
}

func TestDialog04DecodeTransactionResponse(t *testing.T) {
	codec := Dialog04Codec{}
	frame := BuildFrame([]byte("1001250001874"))

	got, err := codec.DecodeTransactionResponse(frame)
	if err != nil {
		t.Fatalf("DecodeTransactionResponse returned error: %v", err)
	}

	want := TransactionResult{StatusCode: "100", WeightGrams: 1250, PriceCents: 1874, RawFrame: frame}
	if got.StatusCode != want.StatusCode || got.WeightGrams != want.WeightGrams || got.PriceCents != want.PriceCents {
		t.Fatalf("DecodeTransactionResponse = %+v, want %+v", got, want)
	}
}

func TestDialog04DecodeTransactionResponseRejectsWrongLength(t *testing.T) {
	codec := Dialog04Codec{}
	frame := BuildFrame([]byte("12345")) // far short of the expected 13
	if _, err := codec.DecodeTransactionResponse(frame); err == nil {
		t.Fatal("expected error for payload with wrong length, got nil")
	}
}

func TestDialog04DecodeTransactionResponseRejectsBadFrame(t *testing.T) {
	codec := Dialog04Codec{}
	frame := BuildFrame([]byte("1001250001874"))
	frame[len(frame)-1] ^= 0xFF // corrupt BCC
	if _, err := codec.DecodeTransactionResponse(frame); err == nil {
		t.Fatal("expected error for frame with bad BCC, got nil")
	}
}

func TestDialog04Name(t *testing.T) {
	if got := (Dialog04Codec{}).Name(); got != "dialog04" {
		t.Fatalf("Name() = %q, want %q", got, "dialog04")
	}
}
