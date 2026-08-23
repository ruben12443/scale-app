package protocol

import "testing"

func TestDialog02EncodeSetPrice(t *testing.T) {
	codec := Dialog02Codec{}
	frame, err := codec.EncodeSetPrice(1499)
	if err != nil {
		t.Fatalf("EncodeSetPrice(1499) returned error: %v", err)
	}
	want := BuildFrame([]byte("01499"))
	if string(frame) != string(want) {
		t.Fatalf("EncodeSetPrice(1499) = % X, want % X", frame, want)
	}
}

func TestDialog02EncodeSetPriceRejectsOutOfRange(t *testing.T) {
	codec := Dialog02Codec{}
	if _, err := codec.EncodeSetPrice(100000); err == nil {
		t.Fatal("expected error for a price that does not fit in 5 digits, got nil")
	}
	if _, err := codec.EncodeSetPrice(-1); err == nil {
		t.Fatal("expected error for a negative price, got nil")
	}
}

func TestDialog02DecodeTransactionResponse(t *testing.T) {
	codec := Dialog02Codec{}
	frame := BuildFrame([]byte("112501874"))

	got, err := codec.DecodeTransactionResponse(frame)
	if err != nil {
		t.Fatalf("DecodeTransactionResponse returned error: %v", err)
	}

	want := TransactionResult{StatusCode: "1", WeightGrams: 1250, PriceCents: 1874, RawFrame: frame}
	if got.StatusCode != want.StatusCode || got.WeightGrams != want.WeightGrams || got.PriceCents != want.PriceCents {
		t.Fatalf("DecodeTransactionResponse = %+v, want %+v", got, want)
	}
}

func TestDialog02DecodeTransactionResponseRejectsWrongLength(t *testing.T) {
	codec := Dialog02Codec{}
	frame := BuildFrame([]byte("1234")) // one digit short of the expected 9
	if _, err := codec.DecodeTransactionResponse(frame); err == nil {
		t.Fatal("expected error for payload with wrong length, got nil")
	}
}

func TestDialog02DecodeTransactionResponseRejectsBadFrame(t *testing.T) {
	codec := Dialog02Codec{}
	frame := BuildFrame([]byte("112501874"))
	frame[len(frame)-1] ^= 0xFF // corrupt BCC
	if _, err := codec.DecodeTransactionResponse(frame); err == nil {
		t.Fatal("expected error for frame with bad BCC, got nil")
	}
}

func TestDialog02Name(t *testing.T) {
	if got := (Dialog02Codec{}).Name(); got != "dialog02" {
		t.Fatalf("Name() = %q, want %q", got, "dialog02")
	}
}
