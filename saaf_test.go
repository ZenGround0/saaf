package saaf_test

import (
	"github.com/zenground0/saaf"
	"testing"
	"fmt"
)

type testNode struct {
	name string
	children []saaf.Pointer
}

func (n *testNode) String() string {
	return n.name
}

func (n *testNode) LongString() string {
	childrenString := "[]"
	if len(n.children) > 0 {
		childrenString := "["
		for _, child := range n.children[:len(n.children)-1] {
			childrenString += fmt.Sprintf("%s,", child)
		}
	}
	
	return fmt.Sprintf("{%s, %s}", n.name, childrenString)
}

func (n *testNode) Children() []saaf.Pointer {
	return n.children
}

func newFromChildren(name string, children []*testNode) *testNode {
	ptrs := make([]saaf.Pointer, len(children))
	for i := range children {
		ptrs[i] = saaf.Pointer(fmt.Sprintf("<%s>", children[i].String()))
	}
	return &testNode{
		name: name,
		children: ptrs,
	}
}

type testResolver struct {
	mapping map[saaf.Pointer]saaf.Node
}

func (r testResolver) add(p saaf.Pointer, n saaf.Node) {
	r.mapping[p] = n
}

func (r testResolver) Resolve(p saaf.Pointer) (saaf.Node, error) {
	n, ok := r.mapping[p]
	if !ok {
		return nil, fmt.Errorf("failed to resolve node at pointer %s", p)
	}
	return n, nil
}



var _ saaf.Resolver = (*testResolver)(nil)
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


func TestLinkUnlinkOneNode(t *testing.T) {
	n := testNode {
		name: "n",
	}
	nPtr := p("<n>")
	r := testResolver{mapping: make(map[saaf.Pointer]saaf.Node)}
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
func testingDAG() ([]*testNode, testResolver) {
	// children
	n2 := newFromChildren("n2", nil)
	n4 := newFromChildren("n4", nil)
	n5 := newFromChildren("n5", nil)
	n6 := newFromChildren("n6", nil)

	// ancestors
	n3 := newFromChildren("n3", []*testNode{n4, n5, n6}) 
	n7 := newFromChildren("n7", []*testNode{n3})
	n1 := newFromChildren("n1", []*testNode{n2, n3})

	// add to resolver
	r := testResolver{mapping: make(map[saaf.Pointer]saaf.Node)}
	r.add(p("<n1>"), n1)
	r.add(p("<n2>"), n2)
	r.add(p("<n3>"), n3)
	r.add(p("<n4>"), n4)
	r.add(p("<n5>"), n5)
	r.add(p("<n6>"), n6)
	r.add(p("<n7>"), n7)

	return []*testNode{n1, n2, n3, n4, n5, n6, n7}, r
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

}
