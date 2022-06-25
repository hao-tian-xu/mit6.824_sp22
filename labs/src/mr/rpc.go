package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
)
import "strconv"

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.

//
// GetTask RPC definitions
//
type GetTaskArgs struct {
	workerId int
}

type GetTaskReply struct {
	taskType TaskType
	taskId   int
	fileName string
	nReduce  int
}

//
// ReportTaskDone RPC definitions
//
type ReportTaskDoneArgs struct {
	taskType TaskType
	taskId   int
	workerId int
}

type ReportTaskDoneReply struct{}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
