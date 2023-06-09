package saaf_test

import (
	"github.com/zenground0/saaf"
	"testing"
	"fmt"
	"sort"
)

type testNode struct {
	name string
	children []saaf.Pointer
}

func (n *testNode) String() string {
	return n.name
}

func (n *testNode) Pointer() saaf.Pointer {
	return saaf.Pointer(fmt.Sprintf("<%s>", n.name))
}

func (n *testNode) LongString() string {
	childrenString := "[]"
	if len(n.children) > 0 {
		childrenString := "["
		for _, child := range n.children[:len(n.children)-1] {
			childrenString += fmt.Sprintf("%s,", child)
		}
	}
	
	return fmt.Sprintf("{%s; %s}", n.name, childrenString)
}

func (n *testNode) Children() []saaf.Pointer {
	return n.children
}

func newFromChildren(name string, children []*testNode) *testNode {
	ptrs := make([]saaf.Pointer, len(children))
	for i := range children {
		ptrs[i] = children[i].Pointer()
	}
	return &testNode{
		name: name,
		children: ptrs,
	}
}

type testSource struct {
	mapping map[saaf.Pointer]saaf.Node
}

func (r testSource) add(p saaf.Pointer, n saaf.Node) {
	r.mapping[p] = n
}

func (r testSource) Resolve(p saaf.Pointer) (saaf.Node, error) {
	n, ok := r.mapping[p]
	if !ok {
		return nil, fmt.Errorf("failed to resolve node at pointer %s", p)
	}
	return n, nil
}



var _ saaf.Source = (*testSource)(nil)
var _ saaf.Node = (*testNode)(nil)

func p(s string) saaf.Pointer {
	return saaf.Pointer(s)
}

func readAll(t *testing.T, ch <-chan saaf.Node) []saaf.Node {
	ns := make([]saaf.Node, 0)
	for n := range ch {
		ns = append(ns, n)
	}
	return ns
}

func assertDAGNodes(t *testing.T, expected []saaf.Node, dag *saaf.DAG) {
	observed := readAll(t, dag.Store().All())
	if summary(observed) != summary(expected) {
		t.Fatalf("expected dag %s but found %s", summary(expected), summary(observed))
	}		
}

// return a comparable summary of a list of nodes by sorting
// and concatenating pointer strings
func summary(ns []saaf.Node) string {
	strs := make([]string, len(ns))
	for i, n := range ns {
		strs[i] = string(n.Pointer())
	}

	sort.Strings(strs)
	ret := ""
	for _, s := range strs {
		ret += "|" + s
	}
	return ret+"|"
}


func TestLinkUnlinkOneNode(t *testing.T) {
	n := testNode {
		name: "n",
	}
	nPtr := p("<n>")
	r := testSource{mapping: make(map[saaf.Pointer]saaf.Node)}
	r.add(nPtr, &n)
	dag := saaf.NewDAG(saaf.NewMapNodeStore())

	// Link
	if err := dag.Link(nPtr, r); err != nil {
		t.Fatal("linking failed")
	}
	nodes := readAll(t, dag.Store().All())
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node in DAG found %d", len(nodes))
	}
	if nodes[0].(*testNode).LongString() != n.LongString() {
		t.Fatal("unexpected out node")
	}

	// Unlink
	if err := dag.Unlink(nPtr); err != nil {
		t.Fatal("unlinking failed")
	}
	nodes = readAll(t, dag.Store().All())
	if len(nodes) != 0 {
		t.Fatalf("expected 0 node in DAG found %d", len(nodes))
	}	
}


/*

  Generate the following DAG
   
        n7  n6
         \  /
     n1--n3--n5
       \   \
        n2  n4

  left to right ancestors to children, i.e. n7->n3->n6, n1->n3->n5:

  return all nodes and resolver
*/
func testingDAG() ([]saaf.Node, testSource) {
	// children

	n2 := saaf.Node(newFromChildren("n2", nil))
	n4 := saaf.Node(newFromChildren("n4", nil))
	n5 := saaf.Node(newFromChildren("n5", nil))
	n6 := saaf.Node(newFromChildren("n6", nil))

	// ancestors
	n3 := saaf.Node(newFromChildren("n3", []*testNode{n4.(*testNode), n5.(*testNode), n6.(*testNode)}))
	n7 := saaf.Node(newFromChildren("n7", []*testNode{n3.(*testNode)}))
	n1 := saaf.Node(newFromChildren("n1", []*testNode{n2.(*testNode), n3.(*testNode)}))

	ns :=  []saaf.Node{n1, n2, n3, n4, n5, n6, n7}
	
	// add to resolver
	src := testSource{mapping: make(map[saaf.Pointer]saaf.Node)}
	for _, n := range ns {
		src.add(n.Pointer(), n.(*testNode))
	}
	
	return ns, src
}


/*
            n7  n6
             \ /
   A --> n1--n3--n5
           \   \
            n2  n4

   Link A
   all but n7 added
   Unlink A
   empty
 */
func TestLinkAddsDAG(t *testing.T) {
	testNodes, src := testingDAG()
	dag := saaf.NewDAG(saaf.NewMapNodeStore())

	// Link A 
	if err := dag.Link(p("<n1>"), src); err != nil {
		t.Fatal("linking failed")
	}
	assertDAGNodes(t, testNodes[:6], dag)

	// Unlink A
	if err := dag.Unlink(p("<n1>")); err != nil {
		t.Fatal("unlinking failed")
	}
	assertDAGNodes(t, nil, dag)
}

/* B ----> n7
            \   n6 
             \ /
   A --> n1--n3--n5
           \   \
            n2  n4

   Link B
   n1, n2 not added
   Link A
   all added
   Unlink A
   n1, n2 deleted
   Unlink B
   empty
 */
func TestLinkSharedSubDAG(t *testing.T) {
	testNodes, src := testingDAG()
	dag := saaf.NewDAG(saaf.NewMapNodeStore())
	A := p("<n1>")
	B := p("<n7>")

	// Link B
	if err := dag.Link(B, src); err != nil {
		t.Fatal("linking B failed")
	}
	assertDAGNodes(t, testNodes[2:], dag)

	// Link A
	if err := dag.Link(A, src); err != nil {
		t.Fatal("linking A failed")
	}
	assertDAGNodes(t, testNodes, dag)

	// Unlink A
	if err := dag.Unlink(A); err != nil {
		t.Fatal("unlinking A failed")
	}
	assertDAGNodes(t, testNodes[2:], dag)

	// Unlink B
	if err := dag.Unlink(B); err != nil {
		t.Fatal("unlinking B failed")
	}
	assertDAGNodes(t, nil, dag)
}

/*         
                n6 <---D
   C--->-----\ /
   A --> n1--n3--n5
           \   \
            n2  n4

   Link D three times
   |n6|
   Link A
   |n1-n6|
   Link C
   |n1-n6|
   

*/
func TestMultiLevelLinking(t *testing.T) {
	testNodes, src := testingDAG()
	dag := saaf.NewDAG(saaf.NewMapNodeStore())
	A := p("<n1>")
	C := p("<n3>")
	D := p("<n6>")

	// Link D three times
	if err := dag.Link(D, src); err != nil {
		t.Fatal("linking D failed")
	}
	assertDAGNodes(t, []saaf.Node{testNodes[5]}, dag)
	if err := dag.Link(D, src); err != nil {
		t.Fatal("linking D failed")
	}
	assertDAGNodes(t, []saaf.Node{testNodes[5]}, dag)
	if err := dag.Link(D, src); err != nil {
		t.Fatal("linking D failed")
	}
	assertDAGNodes(t, []saaf.Node{testNodes[5]}, dag)

	// Link A
	if err := dag.Link(A, src); err != nil {
		t.Fatal("linking A failed")
	}
	assertDAGNodes(t, testNodes[:6], dag)

	// Link C
	if err := dag.Link(C, src); err != nil {
		t.Fatal("linking C failed")
	}
	assertDAGNodes(t, testNodes[:6], dag)	

	// Unlink D twice
	if err := dag.Unlink(D); err != nil {
		t.Fatal("unlinking D failed")
	}
	if err := dag.Unlink(D); err != nil {
		t.Fatal("unlinking D failed")
	}
	assertDAGNodes(t, testNodes[:6], dag)	

	// Unlink D another time
	if err := dag.Unlink(D); err != nil {
		t.Fatal("unlinking D failed")
	}
	assertDAGNodes(t, testNodes[:6], dag)	

	// Unlink A
	if err := dag.Unlink(A); err != nil {
		t.Fatal("unlinking A failed")
	}
	assertDAGNodes(t, testNodes[2:6], dag)	

	// Unlink C
	if err := dag.Unlink(C); err != nil {
		t.Fatal("unlinking C failed")
	}
	assertDAGNodes(t, nil, dag)
}


