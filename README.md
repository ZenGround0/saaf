# saaf
Indirect reference counting for GC of immutable DAGs

## summary
 saaf is a simple library implementing indirect reference counted GC on immutible DAGs                                                                                        
`DAG` `Node`s implement a simple interface using native strings as pointers                                                                                                  
`DAG.Link` pins a node to the DAG, `DAG.Unlink` unpins it.                                                                                                                   
Only linked nodes can be unlinked to disallow dangling references                                                                                                            
`Link` takes in nodes reachable from a root as resolved through a node `Source` and preserves them in a `NodeStore`                                                          
`Unlink` removes nodes from the NodeStore that are no longer linked by any root                                                                                              
          
## scaling
`Link` and `Unlink` operations are intended to scale in node loads and ref updates as O(n + L)                                                                               
 - n is the total number of nodes being added or removed                                                                                                         
 - L is the number of links from the connected component being linked or unlinked pointing into the existing DAG
 
## applications

This library was built with applications over content addressed DAGs directly in mind. In particular efficient snapshots of blockchain state. However it should be general enough to be useful in any place GC of immutible DAGs is needed.
