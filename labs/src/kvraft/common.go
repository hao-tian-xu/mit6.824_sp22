package kvraft

// CONST AND TYPE

const (
	// reply Error Type
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
	// Additional
	ErrDuplicate  = "ErrDuplicate"
	ErrNotApplied = "ErrNotApplied"
	ErrFatal      = "ErrFatal"
	ErrTimeout    = "ErrTimeout"

	// Op type
	opPut    = "Put"
	opAppend = "Append"
	opGet    = "Get"
)

type Err string

// RPC ARGS AND REPLY

//
// Put or Append
//
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
