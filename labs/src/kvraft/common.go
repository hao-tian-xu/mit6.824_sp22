package kvraft

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
	// Additional
	ErrDuplicate   = "ErrDuplicate"
	ErrNotCommited = "ErrNotCommited"
	ErrFatal       = "ErrFatal"

	// Op type
	opPut    = "Put"
	opAppend = "Append"
	opGet    = "Get"
)

type Err string

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	ClientId int
	OpId     int
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.
	ClientId int
	OpId     int
}

type OpReply struct {
	Err   Err
	Value string
}
