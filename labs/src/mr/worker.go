package mr

import (
	"fmt"
	"io/ioutil"
	"os"
)
import "log"
import "net/rpc"
import "hash/fnv"

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	for {
		task, ok := getTask()
		if !ok {
			// TODO
		}

		if task.taskType == MapTask {
			_mapf(mapf, task.fileName, task.id)
			sendTaskDone(task.taskType, task.id)
		} else if task.taskType == ReduceTask {
			_reducef(reducef, task.id)
			sendTaskDone(task.taskType, task.id)
		} else if task.taskType == NoTask {
		}
		// TODO: all tasks finished
		// TODO: task interval
	}

	// archive
	//// TODO: send an RPC to the coordinator asking for a task
	//args := GetTaskArgs{}
	//reply := GetTaskReply{}
	//ok := call("Coordinator.GetTask", &args, &reply)
	//if !ok {
	//	fmt.Printf("call failed!\n")
	//}
	//
	//if reply.Type == MAP {
	//	// TODO: Map task
	//	filename := reply.FileName
	//	file, err := os.Open(filename)
	//	if err != nil {
	//		log.Fatalf("cannot open %v", filename)
	//	}
	//	content, err := ioutil.ReadAll(file)
	//	if err != nil {
	//		log.Fatalf("cannot read %v", filename)
	//	}
	//	file.Close()
	//	mapf(filename, string(content))
	//	// TODO: save intermediate file
	//
	//	// TODO: send intermediate file location to coordinator
	//
	//	// TODO: change task state
	//} else if reply.Type == REDUCE {
	//	// TODO: Reduce task
	//
	//	// TODO: save output file
	//
	//	// TODO: change task state
	//}

	// uncomment to send the Example RPC to the coordinator.
	// CallExample()

}

//
// worker stub
//
func getTask() (*GetTaskReply, bool) {
	args := GetTaskArgs{os.Getpid()}
	reply := GetTaskReply{}
	ok := call("Coordinator.GetTask", &args, &reply)
	if !ok {
		fmt.Printf("get task failed!\n")
	}
	return &reply, ok
}

//
// worker stub
//
func sendTaskDone(taskType TaskType, id int) {

}

//
// process map task and send intermediate file location to coordinator
//
func _mapf(mapf func(string, string) []KeyValue, fileName string, id int) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("cannot open %v", fileName)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", fileName)
	}
	file.Close()
	kva := mapf(fileName, string(content))

	_writeIntermediate(kva, id)
}

func _writeIntermediate(kva []KeyValue, id int) {

}

func _reducef(reducef func(string, []string) string, id int) {

}

//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
