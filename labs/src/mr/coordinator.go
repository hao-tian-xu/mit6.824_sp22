package mr

import (
	"log"
	"sync"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

type Coordinator struct {
	// Your definitions here.
	sync.Mutex
	mapTask      []Task
	reduceTask   []Task
	mapRemain    int
	reduceRemain int
}

// Your code here -- RPC handlers for the worker to call.

//
// Task definition
//
type Task struct {
	taskType  TaskType
	taskState TaskState
	taskId    int
	fileName  string
	workerId  int
}

type TaskType int
type TaskState int

const (
	// Not Applicable
	NA = -1
	// TaskType
	MapTask = iota
	ReduceTask
	NoTask
	Exit
	// TaskState
	Idle
	InProgress
	Completed
)

//
// GetTask RPC handler
//
func (c *Coordinator) GetTask(args *GetTaskArgs, reply *GetTaskReply) error {
	// give a task to the worker
	// TODO: finish map tasks first (no reading from remote disk)
	c.Lock()
	for _, task := range append(c.mapTask, c.reduceTask...) {
		if task.taskState == Idle {
			task.taskState = InProgress
			c.Unlock()
			task.workerId = args.workerId
			reply.taskType = task.taskType
			reply.id = task.taskId
			reply.fileName = task.fileName
			return nil
		}
	}
	// all tasks in process
	reply.taskType = NoTask
	return nil

	// archive
	//if len(c.files) == 0 {
	//	reply.Type = "reduce"
	//	// TODO: reduce Value
	//} else {
	//	reply.Type = MAP
	//	reply.FileName, c.files = c.files[0], c.files[1:]
	//}
}

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here.

	return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}

	// Your code here.
	// initialize task lists
	nMap := len(files)
	mapTask := make([]Task, nMap)
	for i, file := range files {
		mapTask[i] = Task{MapTask, Idle, i, file, NA}
	}
	reduceTask := make([]Task, nReduce)
	for i := 0; i < nReduce; i++ {
		reduceTask[i] = Task{ReduceTask, Idle, i, "", NA}
	}
	c.mapTask, c.reduceTask = mapTask, reduceTask
	c.mapRemain, c.reduceRemain = nMap, nReduce

	c.server()
	return &c
}
