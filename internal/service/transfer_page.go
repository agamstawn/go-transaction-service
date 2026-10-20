package service

// TransferPage contains a page of account-scoped transfers.
type TransferPage struct {
	Data       []TransferResult
	Page       int
	PerPage    int
	Total      int64
	TotalPages int64
}
