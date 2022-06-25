package mr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"
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

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

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
			fmt.Printf("call failed! worker exits\n")
			return
		}
		switch task.taskType {
		case Exit:
			break
		case NoIdle:
		case MapTask:
			_mapf(mapf, task.fileName, task.taskId, task.nReduce)
			reportTaskDone(task.taskType, task.taskId)
		case ReduceTask:
			_reducef(reducef, task.taskId)
			reportTaskDone(task.taskType, task.taskId)
		}
		// task interval
		time.Sleep(time.Millisecond * 100)
	}
	// uncomment to send the Example RPC to the coordinator.
	// CallExample()
}

//
// worker stub: GetTask
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
// worker stub: ReportTaskDone
//
func reportTaskDone(taskType TaskType, taskId int) {
	args := ReportTaskDoneArgs{taskType, taskId, os.Getpid()}
	reply := ReportTaskDoneReply{}
	ok := call("Coordinator.ReportTaskDone", &args, &reply)
	if !ok {
		fmt.Printf("send task done falied!\n")
	}
}

//
// process map task and send intermediate file location to coordinator
//
func _mapf(mapf func(string, string) []KeyValue, fileName string, mapId int, nReduce int) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("cannot open %v", fileName)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", fileName)
	}
	if err := file.Close(); err != nil {
		log.Fatal(err)
	}

	kva := mapf(fileName, string(content))
	writeIntermediateFile(kva, mapId, nReduce)
}

func writeIntermediateFile(kva []KeyValue, mapId int, nReduce int) {
	files := make([]*os.File, nReduce)
	encs := make([]*json.Encoder, nReduce)
	// temp files and encoders
	for i := 0; i < nReduce; i++ {
		file, err := ioutil.TempFile("", "temp.*")
		if err != nil {
			log.Fatal(err)
		}
		files[i] = file
		encs[i] = json.NewEncoder(file)
	}
	// add kv pairs to files
	for _, kv := range kva {
		i := ihash(kv.Key)
		if err := encs[i].Encode(&kv); err != nil {
			log.Fatal(err)
		}
	}
	// atomically rename and close
	for i := 0; i < nReduce; i++ {
		fileName := fmt.Sprintf("mr-%v-%v", mapId, i)
		if err := os.Rename(files[i].Name(), fileName); err != nil {
			log.Fatal(err)
		}
		if err := files[i].Close(); err != nil {
			log.Fatal(err)
		}
	}
}

func _reducef(reducef func(string, []string) string, reduceId int) {

	// read from files
	kva := make([]KeyValue, 0, 100)
	fileNames, err := filepath.Glob(fmt.Sprintf("mr-%v-%v", "*", reduceId))
	if err != nil {
		log.Fatal(err)
	}
	for _, fileName := range fileNames {
		file, err := os.Open(fileName)
		if err != nil {
			log.Fatal(err)
		}
		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			kva = append(kva, kv)
		}
	}

	// sort by key
	sort.Sort(ByKey(kva))

	// write to output file
	ofile, err := ioutil.TempFile("", "temp.*")
	if err != nil {
		log.Fatal(err)
	}
	i := 0
	for i < len(kva) {
		j := i + 1
		for j < len(kva) && kva[j].Key == kva[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, kva[k].Value)
		}
		output := reducef(kva[i].Key, values)
		fmt.Fprintf(ofile, "%v %v\n", kva[i].Key, output)
		i = j
	}
	oname := fmt.Sprintf("mr-out-%v", reduceId)
	os.Rename(ofile.Name(), oname)
	ofile.Close()
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
