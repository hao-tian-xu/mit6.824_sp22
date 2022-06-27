package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type Coordinator struct {
	// Your definitions here.
	mu      sync.Mutex
	task    map[TaskType][]*Task
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
	var task *Task
	c.mu.Lock()
	if c.remain[MapTask] > 0 {
		// map tasks not all done
		task = chooseTask(c.task[MapTask])
	} else if c.remain[ReduceTask] > 0 {
		// reduce tasks not all done
		task = chooseTask(c.task[ReduceTask])
	} else {
		// all tasks done
		c.mu.Unlock()
		reply.TaskType = Exit
		return nil
	}

	if task.taskType == NoIdle {
		// no idle task but some tasks in progress
		c.mu.Unlock()
		reply.TaskType = NoIdle
		return nil
	} else {
		// give a task to the worker
		task.taskState = InProgress
		task.workerId = args.WorkerId
		c.mu.Unlock()
		reply.TaskType = task.taskType
		reply.TaskId = task.taskId
		reply.FileName = task.fileName
		reply.NReduce = c.nReduce
		// wait the task to be completed
		go c.waitTaskDone(task)
		return nil
	}
}

func chooseTask(tasks []*Task) *Task {
	for _, task := range tasks {
		if task.taskState == Idle {
			return task
		}
	}
	return &Task{taskType: NoIdle}
}

func (c *Coordinator) waitTaskDone(task *Task) {
	time.Sleep(time.Second * 10)
	c.mu.Lock()
	defer c.mu.Unlock()
	if task.taskState != Completed {
		log.Printf("worker#%v failed!\n", task.workerId)
		task.taskState = Idle
		task.workerId = NA
	}
}

//
// ReportTaskDone RPC handler
//
func (c *Coordinator) ReportTaskDone(args *ReportTaskDoneArgs, reply *ReportTaskDoneReply) error {

	taskType := args.TaskType
	task := c.task[taskType][args.TaskId]

	// confirm that the task is valid, and mark it completed
	c.mu.Lock()
	defer c.mu.Unlock()
	if task.taskState == InProgress {
		task.taskState = Completed
		c.remain[taskType] -= 1
		log.Printf("task#%v-%v done!\n", task.taskType, task.taskId)
	} else {
		task.taskState = Idle
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
	mapTask := make([]*Task, nMap)
	for i, file := range files {
		mapTask[i] = &Task{MapTask, Idle, i, file, NA}
	}
	reduceTask := make([]*Task, nReduce)
	for i := 0; i < nReduce; i++ {
		reduceTask[i] = &Task{ReduceTask, Idle, i, "", NA}
	}
	c.task = map[TaskType][]*Task{MapTask: mapTask, ReduceTask: reduceTask}
	//c.mapTask, c.reduceTask = mapTask, reduceTask
	c.remain = map[TaskType]int{MapTask: nMap, ReduceTask: nReduce}
	c.nReduce = nReduce

	c.server()
	return &c
}
