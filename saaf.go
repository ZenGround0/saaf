package saaf

import (
	"fmt"
)

// saaf implements indirect reference counting GC on immutible DAGS
// DAG nodes implement a simple interface using native strings as pointers and returning child pointers
// Link and unlink operations scale in node loads as O(n + L) where n is the total number of new nodes being added/removed
// from the DAG, and L is the number of transitive links into connected components separately linked 

type pointer string 

type Node interface {
	Pointer() pointer 
	Children() []pointer 
}

type Resolver interface {
	Resolve(pointer) (Node, error)
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
	refs map[pointer]uint64
	nodes map[pointer]Node 
}

func (d *DAG) link(p pointer, res Resolver) error {
	toLink := []pointer{p}
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
		n, err := res.Resolve(p)
		if err != nil {
			return err
		}
		d.nodes[p] = n
		toLink = append(toLink, n.Children()...)
	}
	return nil
}

func (d *DAG) unlink(p pointer) error {
	toUnlink := []pointer{p}
	for len(toUnlink) > 0 {
		p := toUnlink[0]
		toUnlink = toUnlink[1:]
		r, linked := d.refs[p]
		if !linked {
			return fmt.Errorf("failed to delete pointer %s, node not linked in DAG \n", )
		}
		if r > 1 {
			d.refs[p] -= 1
			continue
		}
		// if this is the last reference delete the sub DAG
		delete(d.refs, p)
		n, ok := d.nodes[p]
		if !ok {
			return fmt.Errorf("internal DAG error, pointer %s reference counted but node not tracked")
		}
		toUnlink = append(toUnlink, n.Children()...)
		delete(d.nodes, p)
	}
	return nil
}

// Selectively pin a series of subDAGs in a DAG by their roots
type LogDAG struct {
	rootSet map[pointer]struct{}
	dag DAG

}

func (ld *LogDAG) apply(p pointer, res Resolver) error {
	// add to rootSet, link
	ld.rootSet[p] = struct{}{}
	return ld.dag.link(p, res)
}

func (ld *LogDAG) revert(p pointer, res Resolver) error {
	// verify in rootSet, unlink
	if _, ok := ld.rootSet[p]; !ok {
		return fmt.Errorf("attempt to revert unpinned subDAG at root %s", p)
	}
	return ld.dag.unlink(p)
}
