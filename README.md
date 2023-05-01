# saaf
Indirect reference counting for GC of immutable DAGs

## summary
 saaf is a simple library implementing indirect reference counted GC on immutible DAGs.  Shared references are counted indirectly on ancestors rather than directly on each node.
 
 `DAG` `Node`s implement a simple interface using native strings as pointers                                                                                                  
`DAG.Link` pins a node to the DAG, `DAG.Unlink` unpins it.                                                                                                                   
`Link` takes in nodes reachable from a root as resolved through a node `Source` and preserves them in a `NodeStore`                                                          
`Unlink` removes nodes from the NodeStore that are no longer linked by any root 

Only nodes linked directly can be unlinked to disallow dangling references                                                                                                            

          
## scaling
`Link` and `Unlink` operations are intended to scale in node loads and ref updates as O(n + L)                                                                               
 - n is the total number of nodes being added or removed                                                                                                         
 - L is the number of links from the connected component being linked or unlinked pointing into the existing DAG

Compared with direct reference counting the advantage is that shared subDAGs are not traversed during (un)linking  
 
## applications

This library was built with applications over content addressed DAGs directly in mind. In particular efficient snapshots of blockchain state. However it should be general enough to be useful in any place GC of immutible DAGs is needed.
