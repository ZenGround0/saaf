package saaf

import (
	"fmt"
)

/*
   saaf is a simple library implementing indirect reference counted GC on immutible DAGs
   `DAG.Link` takes in nodes reachable from a root as resolved through a node `Source` and preserves them in a `NodeStore`
   `DAG.Unlink` removes nodes from the NodeStore that are no longer linked by any root
*/

// saaf implements indirect reference counting GC on immutible DAGS
// DAG functions allow linking and unlinking new subDAGs and access to the collection of linked nodes.
// DAG nodes implement a simple interface using native strings as pointers and returning child pointers
// Link and unlink operations scale in node loads as O(n + L) where n is the total number of new nodes being added/removed
// from the DAG, and L is the number of links from the linked connected component into the existing DAG

type Pointer string 

type Node interface {
	Pointer() Pointer 
	Children() []Pointer
	
}

type Source interface {
	Resolve(Pointer) (Node, error)
}

type NodeStore interface {
	Put(Pointer, Node) error
	Get(Pointer) (Node, error)
	Delete(Pointer) error
	All() <-chan Node
}

// refs tracks reference count of pointers
// 
// count of 0 means pointer is tracked entirely by guard pointer
// count > 0 means pointer is
/*
   
   A -> B -> C
   {A: 1, B: 0, C: 0}

   Q -> A
   {Q: 1, A: 2, B: 0, C: 0}   

   P -> B

   R -> A

   Delete B
   
   Delete R

   choice: dangling delete or no?  if no only delete when ref == 1, if yes delete any
   in both cases delete cnt 1 and traverse all children deleting all that are 0 decrementing all that are 1 recursively

   all linked nodestraversed O(refs) times across all inserts and deletes

   test cases
   1. multiple links req multiple deletes to traverse and delete subgraph, i.e. B -> A->subgraph a, C->A, D->A. subgraph a only deleted when B, C, D deleted
   2 middle of subgraph shared and protected during delete of original linker, i.e. A -> A1 -> A2 (some branching here) -> B -> B1 -> B2 (some branching). Then C -> B, then delete A.  Subgraph A gone but subgraph B stays. Delete C, subgraph B is gone


   shot-snap test cases
   -- we should actually just build snapshot of lotus.
   -- snapshot too big to fit in memory so we'll need to use badger OR get a big machine
   -- badger makes things complicated because deletes need to be GCed, if we use badger we'll need to do a move of valid keys, maybe sorted?
   -- without a big machine we could just restrict ourselves to an actor snapshot, f05 is a good candidate, a few MBs and changing every epoch more or less

   lotus api can maybe notify of head change
   1. given head change we keep asking for blocks over the api, maybe using an api HAMT
   2. once we get to f05 we traverse its head checking for blocks, if we don't have them we ask lotus for them
   3. every time we don't have a block we fetch it and mark as guard reference==1, then use lotus to fetch the whole subtree and add as 0 reference
   4. every time we encounter a block already in the store we increment guard reference.
   5. if notify head tells us about fork updates i.e. deletes / updates we can find the f05 head we're working with and just call delete on it
   
   
*/

type DAG struct {
	// Invariant: alls nodes are tracked in both refs and nodes or neither
	// refs tracks linked references to node at given pointer
	refs map[Pointer]uint64
	// nodes stores all nodes in the DAG
	nodes NodeStore
}

func NewDAG(s NodeStore) *DAG {
	return &DAG{
		refs: make(map[Pointer]uint64),
		nodes: s,
	}
}

func (d *DAG) Link(p Pointer, src Source) error {
	toLink := []Pointer{p}
	for len(toLink) > 0 {
		p := toLink[0]
		toLink = toLink[1:]
		_, linked := d.refs[p]
		if linked {
			d.refs[p] += 1
			continue
		}
		// if not linked then link node and traverse children
		d.refs[p] = 1
		n, err := src.Resolve(p)
		if err != nil {
			return err
		}
		if err := d.nodes.Put(p, n); err != nil {
			fmt.Errorf("failed to put to node store: %w", err)
		}
		toLink = append(toLink, n.Children()...)
	}
	return nil
}

func (d *DAG) Unlink(p Pointer) error {
	toUnlink := []Pointer{p}
	for len(toUnlink) > 0 {
		p := toUnlink[0]
		toUnlink = toUnlink[1:]
		r, linked := d.refs[p]
		if !linked {
			return fmt.Errorf("failed to delete pointer %s, node not linked in DAG \n", p)
		}
		if r > 1 {
			d.refs[p] -= 1
			continue
		}
		// if this is the last reference delete the sub DAG
		delete(d.refs, p)
		n, err := d.nodes.Get(p)
		if err != nil {
			return fmt.Errorf("internal DAG error, pointer %s reference counted but failed to get node: %w", p, err)
		}
		toUnlink = append(toUnlink, n.Children()...)
		if err := d.nodes.Delete(p); err != nil {
			return fmt.Errorf("internal DAG error, failed to delete node %s, %w", p, err)
		}
	}
	return nil
}

func (d *DAG) Store() NodeStore {
	return d.nodes
}

// Selectively pin a series of subDAGs in a DAG by their roots
type LogDAG struct {
	rootSet map[Pointer]struct{}
	dag DAG

}

func (ld *LogDAG) apply(p Pointer, src Source) error {
	// add to rootSet, link
	ld.rootSet[p] = struct{}{}
	return ld.dag.Link(p, src)
}

func (ld *LogDAG) revert(p Pointer, src Source) error {
	// verify in rootSet, unlink
	if _, ok := ld.rootSet[p]; !ok {
		return fmt.Errorf("attempt to revert unpinned subDAG at root %s", p)
	}
	return ld.dag.Unlink(p)
}

// In memory node store backed by a simple map
type MapNodeStore struct {
	nodes map[Pointer]Node
}

func NewMapNodeStore() MapNodeStore {
	return MapNodeStore{
		nodes: make(map[Pointer]Node),
	}
}

func (s MapNodeStore) Put(p Pointer, n Node) error {
	s.nodes[p] = n
	return nil
}

func (s MapNodeStore) Get(p Pointer) (Node, error) {
	n, ok := s.nodes[p]
	if !ok {
		return nil, fmt.Errorf("could not resolve pointer %s", p)
	}
	return n, nil
}

func (s MapNodeStore) All() <-chan Node {
	ch := make(chan Node, 0)
	go func() {
		for p := range s.nodes {
			ch <- s.nodes[p]
		}
		close(ch)
	}()
	return ch
}

func (s MapNodeStore) Delete(p Pointer) error {
	if _, ok := s.nodes[p]; !ok {
		return fmt.Errorf("%s not stored", p)
	}
	delete(s.nodes, p)
	return nil
}

var _ NodeStore = MapNodeStore{}
