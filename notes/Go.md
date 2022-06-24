# A TOUR OF GO

## Packages, variables, and functions

### Packages

- Exported names
  - In Go, a name is exported if it begins with a capital letter.

When importing a package, you can refer only to its exported names. Any "unexported" names are not accessible from outside the package.

### Functions

- Declaration syntax
  - the type comes *after* the variable name
  - you can omit the type from all but the last
  - see the [article on Go's declaration syntax](https://blog.golang.org/gos-declaration-syntax)
- "Naked" return
  - A `return` statement without arguments returns the named return values.

### Variables

- `var`
  - The `var` statement declares a list of variables; as in function argument lists, the type is last.
- Initializers
  - If an initializer is present, the type can be omitted
- `:=`
  - Inside a function, the `:=` short assignment statement can be used in place of a `var` declaration with implicit type.
  - Outside a function, every statement begins with a keyword
- Basic types
- Zero values
  - `0`, `false`, `""`

## Flow control statements: for, if, else, switch and defer

### For

```go
for i := 0; i < 10; i++ {
  sum += i
}
```

- The init and post statements are optional.
  - At that point you can drop the semicolons: C's `while` is spelled `for` in Go.
- If you omit the loop condition it loops forever, so an infinite loop is compactly expressed.

### If

```go
if v := math.Pow(x, n); v < lim {
  return v
} else {
  fmt.Printf("%g >= %g\n", v, lim)
}
```

### Switch

```go
switch time.Saturday {
case today + 0:
	fmt.Println("Today.")
case today + 1:
	fmt.Println("Tomorrow.")
case today + 2:
	fmt.Println("In two days.")
default:
	fmt.Println("Too far away.")
}
```

- Switch without a condition is the same as `switch true`.

### Defer

```go
func main() {
	fmt.Println("counting")

	for i := 0; i < 10; i++ {
		defer fmt.Println(i)
	}

	fmt.Println("done")
}
```

## More types: structs, slices, and maps

### Pointers

- The type `*T` is a pointer to a `T` value. Its zero value is `nil`.

  ```
  var p *int
  ```

- The `&` operator generates a pointer to its operand.

  ```
  i := 42
  p = &i
  ```

- The `*` operator denotes the pointer's underlying value.

  ```
  fmt.Println(*p) // read i through the pointer p
  *p = 21         // set i through the pointer p
  ```

### Structs

- A `struct` is a collection of fields.

  - Struct fields are accessed using a dot.
  - To access the field `X` of a struct when we have the struct pointer `p` we could write `(*p).X`. However, that notation is cumbersome, so the language permits us instead to write just `p.X`, without the explicit dereference.

- Struct Literals

  - ```go
    type Vertex struct {
    	X, Y int
    }
    
    var (
    	v1 = Vertex{1, 2}  // has type Vertex
    	v2 = Vertex{X: 1}  // Y:0 is implicit
    	v3 = Vertex{}      // X:0 and Y:0
    	p  = &Vertex{1, 2} // has type *Vertex
    )
    ```

### Arrays

```go
var a [2]string
a[0] = "Hello"

// Slices
primes := [6]int{2, 3, 5, 7, 11, 13}
var s []int = primes[1:4]
// Slices are like references to arrays
// literals
a := [3]bool{true, true, false}		// array literal
s := []bool{true, true, false}		// slice literal
// Slice length and capacity
// Nil slices
// Creating a slice with make
b := make([]int, 0, 5)
// Slices can contain any type, including other slices.
// Appending to a slice
func append(s []T, vs ...T) []T
```

### Range

```go
var pow = []int{1, 2, 4, 8, 16, 32, 64, 128}
for i, v := range pow {
  fmt.Printf("2**%d = %d\n", i, v)
}		// If you only want the index, you can omit the second variable.
```

### Maps

```go
type Vertex struct {
	Lat, Long float64
}

m := make(map[string]Vertex)
m["Bell Labs"] = Vertex{
  40.68433, -74.39967,
}

// Map literals
var m = map[string]Vertex{
	"Bell Labs": {40.68433, -74.39967},
	"Google":    {37.42202, -122.08408},
}
// Delete
delete(m, key)
// Test that a key is present with a two-value assignment:
elem, ok = m[key]		// If key is in m, ok is true. If not, ok is false.
```

### Function values

```go
// Functions are values too. They can be passed around just like other values
// Function closures
	// A closure is a function value that references variables from outside its body.
	// In this sense the function is "bound" to the variables.
func adder() func(int) int {
	sum := 0
	return func(x int) int {
		sum += x
		return sum
	}
}
```

## Methods and interfaces

### Methods

- Go does not have classes. However, you can define methods on types.
- A method is a function with a special *receiver* argument.

```go
type Vertex struct {
	X, Y float64
}

func (v Vertex) Abs() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}

// You can declare a method on non-struct types, too.
type MyFloat float64

func (f MyFloat) Abs() float64 {
	if f < 0 {
		return float64(-f)
	}
	return float64(f)
}
```

### Pointer receivers

- In general, all methods on a given type should have either value or pointer receivers

### Interfaces

```go
// An *interface type* is defined as a set of method signatures.
	// A value of interface type can hold any value that implements those methods.
// Interfaces are implemented implicitly
type I interface {
	M()
}

type T struct {
	S string
}

func (t T) M() {
	fmt.Println(t.S)
}

// Interface values with nil underlying values

// The empty interface
	// Empty interfaces are used by code that handles values of unknown type.
func main() {
	var i interface{}
	describe(i)

	i = 42
	describe(i)

	i = "hello"
	describe(i)
}

func describe(i interface{}) {
	fmt.Printf("(%v, %T)\n", i, i)
}

// Type assertions
t := i.(T)		// This statement asserts that the interface value i holds the concrete type T and assigns the underlying T value to the variable t. If i does not hold a T, the statement will trigger a panic.
t, ok := i.(T)

// Type switches
func do(i interface{}) {
	switch v := i.(type) {
	case int:
		fmt.Printf("Twice %v is %v\n", v, v*2)
	case string:
		fmt.Printf("%q is %v bytes long\n", v, len(v))
	default:
		fmt.Printf("I don't know about type %T!\n", v)
	}
}

// Stringers
type Stringer interface {
    String() string
}

// Errors
type error interface {
    Error() string
}
	// Functions often return an error value, and calling code should handle errors by testing whether the error equals nil.
i, err := strconv.Atoi("42")
if err != nil {
    fmt.Printf("couldn't convert number: %v\n", err)
    return
}

// Readers
	// The io.Reader interface has a Read method:
func (T) Read(b []byte) (n int, err error)
	// e.g.
func main() {
	r := strings.NewReader("Hello, Reader!")

	b := make([]byte, 8)
	for {
		n, err := r.Read(b)
		fmt.Printf("n = %v err = %v b = %v\n", n, err, b)
		fmt.Printf("b[:n] = %q\n", b[:n])
		if err == io.EOF {
			break
		}
	}
}
```

### Images

```go
package image

type Image interface {
    ColorModel() color.Model
    Bounds() Rectangle
    At(x, y int) color.Color
}
```

## Generics

```go
// The type parameters of a function appear between brackets, before the function's arguments.
func Index[T comparable](s []T, x T) int		// This declaration means that s is a slice of any type T that fulfills the built-in constraint comparable. x is also a value of the same type.

// Generic types
type List[T any] struct {
	next *List[T]
	val  T
}


```

## Concurrency

### Goroutines

```go
// A goroutine is a lightweight thread managed by the Go runtime.
go f(x, y, z)
	// Goroutines run in the same address space, so access to shared memory must be synchronized.
```

### Channels

```go
// Channels are a typed conduit through which you can send and receive values with the channel operator, <-
ch := make(chan int)
ch <- v    // Send v to channel ch.
v := <-ch  // Receive from ch, andassign value to v.
	// By default, sends and receives block until the other side is ready. This allows goroutines to synchronize without explicit locks or condition variables.

// Buffered Channels
ch := make(chan int, 100)		// the buffer length as the second argument to make

// Range and Close
close(ch)
v, ok := <-ch		// ok is false if there are no more values to receive and the channel is closed.
								// Channels aren't like files; you don't usually need to close them.

// Select
	// The select statement lets a goroutine wait on multiple communication operations.
	// Default Selection
		// The default case in a select is run if no other case is ready.
select {
case i := <-c:
    // use i
default:
    // receiving from c would block
}
```

### sync.Mutex

```go
// Go's standard library provides mutual exclusion with sync.Mutex and its two methods: Lock, Unlock
	// We can also use defer to ensure the mutex will be unlocked
type SafeCounter struct {
	mu sync.Mutex
	v  map[string]int
}

func (c *SafeCounter) Inc(key string) {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.v[key]++
	c.mu.Unlock()
}

func (c *SafeCounter) Value(key string) int {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mu.Unlock()
	return c.v[key]
}

```





























# EXERCISE

## Flow control statements: for, if, else, switch and defer

### Loops and Functions

```go
package main

import (
	"fmt"
)

func Sqrt(x float64) float64 {
	z := 1.0
	for {
		old := z
		z -= (z*z - x) / (2 * z)
		if z == old {
			return z
		}
	}
}

func main() {
	fmt.Println(Sqrt(5))
}

```



## More types: structs, slices, and maps

### Slices

```go
package main

import "golang.org/x/tour/pic"

func Pic(dx, dy int) [][]uint8 {
	result := make([][]uint8, dy)
	for y := 0; y < dy; y++ {
		result[y] = make([]uint8, dx)
		for x := 0; x < dx; x++ {
			result[y][x] = uint8((x + y) / 2)
		}
	}
	return result
}

func main() {
	pic.Show(Pic)
}

```

### Maps

```go
package main

import (
	"strings"

	"golang.org/x/tour/wc"
)

func WordCount(s string) map[string]int {
	result := make(map[string]int)

	words := strings.Fields(s)
	for _, word := range words {
		result[word] += 1
	}

	return result
}

func main() {
	wc.Test(WordCount)
}

```

### Fibonacci closure

```go
package main

import "fmt"

// fibonacci is a function that returns
// a function that returns an int.
func fibonacci() func() int {
	a, b := 0, 1
	return func() int {
		temp := a
		a = b
		b += temp
		return temp
	}
}

func main() {
	f := fibonacci()
	for i := 0; i < 10; i++ {
		fmt.Println(f())
	}
}

```



## Methods and interfaces

### Stringers

```go
package main

import "fmt"

type IPAddr [4]byte

// TODO: Add a "String() string" method to IPAddr.
func (i IPAddr) String() string {
	return fmt.Sprintf("%v.%v.%v.%v", i[0], i[1], i[2], i[3])
}

func main() {
	hosts := map[string]IPAddr{
		"loopback":  {127, 0, 0, 1},
		"googleDNS": {8, 8, 8, 8},
	}
	for name, ip := range hosts {
		fmt.Printf("%v: %v\n", name, ip)
	}
}

```

### Errors

```go
package main

import (
	"fmt"
)

type ErrNegativeSqrt float64

func (e ErrNegativeSqrt) Error() string {
	return fmt.Sprintf("cannot Sqrt negative number: %v", float64(e))
}

func Sqrt(x float64) (float64, error) {
	if x < 0 {
		return 0, ErrNegativeSqrt(x)
	}

	z := 1.0
	for i := 0; i < 10; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z, nil
}

func main() {
	fmt.Println(Sqrt(2))
	fmt.Println(Sqrt(-2))
}

```

### Readers

```go
package main

import "golang.org/x/tour/reader"

type MyReader struct{}

// TODO: Add a Read([]byte) (int, error) method to MyReader.
func (r MyReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 'A'
	}
	return len(b), nil
}

func main() {
	reader.Validate(MyReader{})
}

```

### rot13Reader

```go
package main

import (
	"io"
	"os"
	"strings"
)

type rot13Reader struct {
	r io.Reader
}

func (rot13 rot13Reader) Read(b []byte) (n int, e error) {
	n, e = rot13.r.Read(b)
	for i, v := range b {
		if (v >= 'a' && v <= 'm') || (v >= 'A' && v <= 'M')  {
			b[i] += 13;
		} else if (v >= 'n' && v <= 'z') || (v >= 'N' && v <= 'Z') {
			b[i] -= 13;
		}
	}
	return
}

func main() {
	s := strings.NewReader("Lbh penpxrq gur pbqr!")
	r := rot13Reader{s}
	io.Copy(os.Stdout, &r)
}

```

### Images

```go
package main

import (
	"image"
	"image/color"

	"golang.org/x/tour/pic"
)

type Image struct{}

func (i Image) Bounds() image.Rectangle {
	return image.Rect(0, 0, 256, 256)
}

func (i Image) At(x, y int) color.Color {
	v := uint8(x * y)
	return color.RGBA{v, v, 255, 255}
}

func (i Image) ColorModel() color.Model {
	return color.RGBAModel
}

func main() {
	m := Image{}
	pic.ShowImage(m)
}

```



## Concurrency

### Equivalent Binary Trees

```go
package main

import (
	"fmt"

	"golang.org/x/tour/tree"
)

// Walk walks the tree t sending all values
// from the tree to the channel ch.
func Walk(t *tree.Tree, ch chan int) {
	_walk(t, ch)
	close(ch)
}

// recursion helper
func _walk(t *tree.Tree, ch chan int) {
	if t == nil {
		return
	}
	_walk(t.Left, ch)
	ch <- t.Value
	_walk(t.Right, ch)
}

// Same determines whether the trees
// t1 and t2 contain the same values.
func Same(t1, t2 *tree.Tree) bool {
	ch1, ch2 := make(chan int), make(chan int)
	go Walk(t1, ch1)
	go Walk(t2, ch2)
	for {
		v1, ok1 := <-ch1
		v2, ok2 := <-ch2
		if (v1 != v2) || (ok1 != ok2) {
			return false
		}
		if !ok1 {
			break
		}
	}
	return true
}

func main() {
	// Walk test
	ch := make(chan int)
	go Walk(tree.New(1), ch)
	for {
		v, ok := <-ch
		if !ok {
			break
		}
		fmt.Printf("%v ", v)
	}
	println()
	// Same test
	println(Same(tree.New(1), tree.New(1)))
	println(Same(tree.New(1), tree.New(2)))
}

```

### Web Crawler

```go
package main

import (
	"fmt"
	"sync"
)

type Fetcher interface {
	// Fetch returns the body of URL and
	// a slice of URLs found on that page.
	Fetch(url string) (body string, urls []string, err error)
}

// Crawl uses fetcher to recursively crawl
// pages starting with url, to a maximum of depth.
type SafeMap struct {
	mu sync.Mutex
	m  map[string]bool
}

var fetched = SafeMap{m: make(map[string]bool)}

func Crawl(url string, depth int, fetcher Fetcher) {
	// TODO: Fetch URLs in parallel.
	// TODO: Don't fetch the same URL twice.
	// This implementation doesn't do either:
	if depth <= 0 {
		return
	}

	fetched.mu.Lock()
	if fetched.m[url] {
		defer fetched.mu.Unlock()
		return
	}
	fetched.m[url] = true
	fetched.mu.Unlock()

	body, urls, err := fetcher.Fetch(url)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("found: %s %q\n", url, body)

	done := make(chan bool)
	for _, u := range urls {
		go func(url string) {
			Crawl(url, depth-1, fetcher)
			done <- true
		}(u)
	}
	for i := 0; i < len(urls); i++ {
		<-done
	}
	return
}

func main() {
	Crawl("https://golang.org/", 4, fetcher)
}

// fakeFetcher is Fetcher that returns canned results.
type fakeFetcher map[string]*fakeResult

type fakeResult struct {
	body string
	urls []string
}

func (f fakeFetcher) Fetch(url string) (string, []string, error) {
	if res, ok := f[url]; ok {
		return res.body, res.urls, nil
	}
	return "", nil, fmt.Errorf("not found: %s", url)
}

// fetcher is a populated fakeFetcher.
var fetcher = fakeFetcher{
	"https://golang.org/": &fakeResult{
		"The Go Programming Language",
		[]string{
			"https://golang.org/pkg/",
			"https://golang.org/cmd/",
		},
	},
	"https://golang.org/pkg/": &fakeResult{
		"Packages",
		[]string{
			"https://golang.org/",
			"https://golang.org/cmd/",
			"https://golang.org/pkg/fmt/",
			"https://golang.org/pkg/os/",
		},
	},
	"https://golang.org/pkg/fmt/": &fakeResult{
		"Package fmt",
		[]string{
			"https://golang.org/",
			"https://golang.org/pkg/",
		},
	},
	"https://golang.org/pkg/os/": &fakeResult{
		"Package os",
		[]string{
			"https://golang.org/",
			"https://golang.org/pkg/",
		},
	},
}

```

