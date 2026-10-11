package chat

// MemoryInterface defines dialogue memory management interface
type MemoryInterface interface {
	// QueryMemory queries relevant memory
	QueryMemory(query string) (string, error)

	// SaveMemory saves dialogue memory
	SaveMemory(dialogue []Message) error

	// ClearMemory clears memory
	ClearMemory() error
}
