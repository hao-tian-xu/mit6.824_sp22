package mr

import (
	"log"
	"sync"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

type Coordinator struct {
	// Your definitions here.
	sync.Mutex
	task map[TaskType][]Task
	//mapTask      []Task
	//reduceTask   []Task
	remain  map[TaskType]int
	nReduce int
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
	NoIdle
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

	// finish map tasks first, reasons:
	// 1. no reading from remote disk
	// 2. number of workers may not be over nReduce, thus failed map task may not have any worker to work on
	c.Lock()
	if c.remain[MapTask] > 0 {
		// map tasks not all done
		c.giveTask(args, reply, MapTask)
	} else if c.remain[ReduceTask] > 0 {
		// reduce tasks not all done
		c.giveTask(args, reply, ReduceTask)
	} else {
		// all tasks done
		c.Unlock()
		reply.taskType = Exit
	}

	// wait for the task to be done after return
	taskType := reply.taskType
	taskId := reply.taskId
	defer func() {
		if taskType == MapTask || taskType == ReduceTask {
			time.Sleep(time.Second * 10)
			task := c.task[taskType][taskId]
			c.Lock()
			if task.taskState != Completed {
				task.taskState = Idle
			}
			c.Unlock()
		}
	}()
	return nil
}

func (c *Coordinator) giveTask(args *GetTaskArgs, reply *GetTaskReply, taskType TaskType) {

	// find next idle task
	task := Task{}
	taskZero := task
	for _, t := range c.task[taskType] {
		if t.taskState == Idle {
			task = t
			break
		}
	}

	// reply the task
	if task != taskZero {
		task.taskState = InProgress
		c.Unlock()
		task.workerId = args.workerId
		reply.taskType = task.taskType
		reply.taskId = task.taskId
		reply.fileName = task.fileName
		reply.nReduce = c.nReduce
		return
	}

	// no idle task but some tasks in progress
	c.Unlock()
	reply.taskType = NoIdle
}

//
// ReportTaskDone RPC handler
//
func (c *Coordinator) ReportTaskDone(args *ReportTaskDoneArgs, reply *ReportTaskDoneReply) error {

	taskType := args.taskType
	task := c.task[taskType][args.taskId]

	// confirm that the task is valid, and mark it completed
	c.Lock()
	defer c.Unlock()
	if task.taskState == InProgress && task.workerId == args.workerId {
		task.taskState = Completed
		c.remain[taskType] -= 1
	}

	return nil
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
	return c.remain[MapTask] <= 0 && c.remain[ReduceTask] <= 0
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
	c.task = map[TaskType][]Task{MapTask: mapTask, ReduceTask: reduceTask}
	//c.mapTask, c.reduceTask = mapTask, reduceTask
	c.remain = map[TaskType]int{MapTask: nMap, ReduceTask: nReduce}
	c.nReduce = nReduce

	c.server()
	return &c
}
