let wasm;

function addToExternrefTable0(obj) {
    const idx = wasm.__externref_table_alloc();
    wasm.__wbindgen_externrefs.set(idx, obj);
    return idx;
}

function getArrayU8FromWasm0(ptr, len) {
    ptr = ptr >>> 0;
    return getUint8ArrayMemory0().subarray(ptr / 1, ptr / 1 + len);
}

let cachedDataViewMemory0 = null;
function getDataViewMemory0() {
    if (cachedDataViewMemory0 === null || cachedDataViewMemory0.buffer.detached === true || (cachedDataViewMemory0.buffer.detached === undefined && cachedDataViewMemory0.buffer !== wasm.memory.buffer)) {
        cachedDataViewMemory0 = new DataView(wasm.memory.buffer);
    }
    return cachedDataViewMemory0;
}

function getStringFromWasm0(ptr, len) {
    ptr = ptr >>> 0;
    return decodeText(ptr, len);
}

let cachedUint32ArrayMemory0 = null;
function getUint32ArrayMemory0() {
    if (cachedUint32ArrayMemory0 === null || cachedUint32ArrayMemory0.byteLength === 0) {
        cachedUint32ArrayMemory0 = new Uint32Array(wasm.memory.buffer);
    }
    return cachedUint32ArrayMemory0;
}

let cachedUint8ArrayMemory0 = null;
function getUint8ArrayMemory0() {
    if (cachedUint8ArrayMemory0 === null || cachedUint8ArrayMemory0.byteLength === 0) {
        cachedUint8ArrayMemory0 = new Uint8Array(wasm.memory.buffer);
    }
    return cachedUint8ArrayMemory0;
}

function handleError(f, args) {
    try {
        return f.apply(this, args);
    } catch (e) {
        const idx = addToExternrefTable0(e);
        wasm.__wbindgen_exn_store(idx);
    }
}

function isLikeNone(x) {
    return x === undefined || x === null;
}

function passArray32ToWasm0(arg, malloc) {
    const ptr = malloc(arg.length * 4, 4) >>> 0;
    getUint32ArrayMemory0().set(arg, ptr / 4);
    WASM_VECTOR_LEN = arg.length;
    return ptr;
}

function passArray8ToWasm0(arg, malloc) {
    const ptr = malloc(arg.length * 1, 1) >>> 0;
    getUint8ArrayMemory0().set(arg, ptr / 1);
    WASM_VECTOR_LEN = arg.length;
    return ptr;
}

function passStringToWasm0(arg, malloc, realloc) {
    if (realloc === undefined) {
        const buf = cachedTextEncoder.encode(arg);
        const ptr = malloc(buf.length, 1) >>> 0;
        getUint8ArrayMemory0().subarray(ptr, ptr + buf.length).set(buf);
        WASM_VECTOR_LEN = buf.length;
        return ptr;
    }

    let len = arg.length;
    let ptr = malloc(len, 1) >>> 0;

    const mem = getUint8ArrayMemory0();

    let offset = 0;

    for (; offset < len; offset++) {
        const code = arg.charCodeAt(offset);
        if (code > 0x7F) break;
        mem[ptr + offset] = code;
    }
    if (offset !== len) {
        if (offset !== 0) {
            arg = arg.slice(offset);
        }
        ptr = realloc(ptr, len, len = offset + arg.length * 3, 1) >>> 0;
        const view = getUint8ArrayMemory0().subarray(ptr + offset, ptr + len);
        const ret = cachedTextEncoder.encodeInto(arg, view);

        offset += ret.written;
        ptr = realloc(ptr, len, offset, 1) >>> 0;
    }

    WASM_VECTOR_LEN = offset;
    return ptr;
}

function takeFromExternrefTable0(idx) {
    const value = wasm.__wbindgen_externrefs.get(idx);
    wasm.__externref_table_dealloc(idx);
    return value;
}

let cachedTextDecoder = new TextDecoder('utf-8', { ignoreBOM: true, fatal: true });
cachedTextDecoder.decode();
const MAX_SAFARI_DECODE_BYTES = 2146435072;
let numBytesDecoded = 0;
function decodeText(ptr, len) {
    numBytesDecoded += len;
    if (numBytesDecoded >= MAX_SAFARI_DECODE_BYTES) {
        cachedTextDecoder = new TextDecoder('utf-8', { ignoreBOM: true, fatal: true });
        cachedTextDecoder.decode();
        numBytesDecoded = len;
    }
    return cachedTextDecoder.decode(getUint8ArrayMemory0().subarray(ptr, ptr + len));
}

const cachedTextEncoder = new TextEncoder();

if (!('encodeInto' in cachedTextEncoder)) {
    cachedTextEncoder.encodeInto = function (arg, view) {
        const buf = cachedTextEncoder.encode(arg);
        view.set(buf);
        return {
            read: arg.length,
            written: buf.length
        };
    }
}

let WASM_VECTOR_LEN = 0;

const DiGraphFinalization = (typeof FinalizationRegistry === 'undefined')
    ? { register: () => {}, unregister: () => {} }
    : new FinalizationRegistry(ptr => wasm.__wbg_digraph_free(ptr >>> 0, 1));

/**
 * Directed graph optimized for graph algorithms.
 * Uses adjacency lists for O(1) neighbor access.
 */
export class DiGraph {
    static __wrap(ptr) {
        ptr = ptr >>> 0;
        const obj = Object.create(DiGraph.prototype);
        obj.__wbg_ptr = ptr;
        DiGraphFinalization.register(obj, obj.__wbg_ptr, obj);
        return obj;
    }
    __destroy_into_raw() {
        const ptr = this.__wbg_ptr;
        this.__wbg_ptr = 0;
        DiGraphFinalization.unregister(this);
        return ptr;
    }
    free() {
        const ptr = this.__destroy_into_raw();
        wasm.__wbg_digraph_free(ptr, 0);
    }
    /**
     * Get the degeneracy of the graph (maximum core number).
     * @returns {number}
     */
    degeneracy() {
        const ret = wasm.digraph_degeneracy(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Get direct dependents (successors) of a node.
     * These are issues that depend on this node being completed.
     * @param {number} node
     * @returns {any}
     */
    dependents(node) {
        const ret = wasm.digraph_dependents(this.__wbg_ptr, node);
        return ret;
    }
    /**
     * Number of edges.
     * @returns {number}
     */
    edgeCount() {
        const ret = wasm.digraph_edgeCount(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Check if graph has any cycles.
     * @returns {boolean}
     */
    hasCycles() {
        const ret = wasm.digraph_hasCycles(this.__wbg_ptr);
        return ret !== 0;
    }
    /**
     * All in-degrees as a vector (JSON array).
     * @returns {any}
     */
    inDegrees() {
        const ret = wasm.digraph_inDegrees(this.__wbg_ptr);
        return ret;
    }
    /**
     * Number of nodes.
     * @returns {number}
     */
    nodeCount() {
        const ret = wasm.digraph_nodeCount(this.__wbg_ptr);
        return ret >>> 0;
    }
    /**
     * Out-degree of a node (number of dependencies).
     * @param {number} node
     * @returns {number}
     */
    outDegree(node) {
        const ret = wasm.digraph_outDegree(this.__wbg_ptr, node);
        return ret >>> 0;
    }
    /**
     * Get successors of a node as JSON array of indices.
     * @param {number} node
     * @returns {any}
     */
    successors(node) {
        const ret = wasm.digraph_successors(this.__wbg_ptr, node);
        return ret;
    }
    /**
     * Find strongly connected components using Tarjan's algorithm.
     * Returns JSON: { components: number[][], has_cycles: bool, cycle_count: number }
     * @returns {any}
     */
    tarjanScc() {
        const ret = wasm.digraph_tarjanScc(this.__wbg_ptr);
        return ret;
    }
    /**
     * All issues with cascade impact, sorted by impact.
     * Considers all open nodes (not just actionable).
     * Returns JSON array of {node, result} sorted by transitive_unblocks.
     * @param {Uint8Array} closed_set
     * @param {number} limit
     * @returns {any}
     */
    allWhatIf(closed_set, limit) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_allWhatIf(this.__wbg_ptr, ptr0, len0, limit);
        return ret;
    }
    /**
     * Compute exact betweenness centrality using Brandes' algorithm.
     * Returns array of scores in node index order.
     * Complexity: O(V*E) - use betweenness_approx for large graphs.
     * @returns {any}
     */
    betweenness() {
        const ret = wasm.digraph_betweenness(this.__wbg_ptr);
        return ret;
    }
    /**
     * Compute eigenvector centrality using power iteration.
     * Returns array of scores in node index order, normalized to unit length.
     * @param {number} iterations
     * @returns {any}
     */
    eigenvector(iterations) {
        const ret = wasm.digraph_eigenvector(this.__wbg_ptr, iterations);
        return ret;
    }
    /**
     * All out-degrees as a vector (JSON array).
     * @returns {any}
     */
    outDegrees() {
        const ret = wasm.digraph_outDegrees(this.__wbg_ptr);
        return ret;
    }
    /**
     * Top N issues by cascade impact.
     * Only considers currently actionable nodes.
     * Returns JSON array of {node, result} sorted by transitive_unblocks.
     * @param {Uint8Array} closed_set
     * @param {number} limit
     * @returns {any}
     */
    topWhatIf(closed_set, limit) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_topWhatIf(this.__wbg_ptr, ptr0, len0, limit);
        return ret;
    }
    /**
     * Get the total float (maximum slack) in the graph.
     * @returns {number}
     */
    totalFloat() {
        const ret = wasm.digraph_totalFloat(this.__wbg_ptr);
        return ret;
    }
    /**
     * Compute coverage set (greedy vertex cover).
     * Finds nodes that collectively "cover" all edges in the graph.
     * Returns JSON: { items: [{node, edges_added}], edges_covered, total_edges, coverage_ratio }
     * @param {number} limit
     * @returns {any}
     */
    coverageSet(limit) {
        const ret = wasm.digraph_coverageSet(this.__wbg_ptr, limit);
        return ret;
    }
    /**
     * Compute HITS with default parameters (tolerance=1e-6, max_iterations=100).
     * @returns {any}
     */
    hitsDefault() {
        const ret = wasm.digraph_hitsDefault(this.__wbg_ptr);
        return ret;
    }
    /**
     * Get predecessors of a node as JSON array of indices.
     * @param {number} node
     * @returns {any}
     */
    predecessors(node) {
        const ret = wasm.digraph_predecessors(this.__wbg_ptr, node);
        return ret;
    }
    /**
     * Get all node indices that can reach a target node (incoming direction).
     * @param {number} target
     * @returns {any}
     */
    reachableTo(target) {
        const ret = wasm.digraph_reachableTo(this.__wbg_ptr, target);
        return ret;
    }
    /**
     * Get open blockers for a node (predecessors not in closed_set).
     * closed_set is an array of bytes where non-zero means closed.
     * @param {number} node
     * @param {Uint8Array} closed_set
     * @returns {any}
     */
    openBlockers(node, closed_set) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_openBlockers(this.__wbg_ptr, node, ptr0, len0);
        return ret;
    }
    /**
     * What-if analysis: compute cascade impact of closing a node.
     * Returns JSON with direct_unblocks, transitive_unblocks, unblocked_ids, cascade_ids, parallel_gain.
     * closed_set is an array of bytes where non-zero means closed.
     * @param {number} node
     * @param {Uint8Array} closed_set
     * @returns {any}
     */
    whatIfClose(node, closed_set) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_whatIfClose(this.__wbg_ptr, node, ptr0, len0);
        return ret;
    }
    /**
     * Create a graph with pre-allocated capacity.
     * @param {number} node_capacity
     * @param {number} edge_capacity
     * @returns {DiGraph}
     */
    static withCapacity(node_capacity, edge_capacity) {
        const ret = wasm.digraph_withCapacity(node_capacity, edge_capacity);
        return DiGraph.__wrap(ret);
    }
    /**
     * Get just the node indices from coverage set computation.
     * @param {number} limit
     * @returns {any}
     */
    coverageNodes(limit) {
        const ret = wasm.digraph_coverageNodes(this.__wbg_ptr, limit);
        return ret;
    }
    /**
     * Get all node indices reachable from a source node (outgoing direction).
     * @param {number} source
     * @returns {any}
     */
    reachableFrom(source) {
        const ret = wasm.digraph_reachableFrom(this.__wbg_ptr, source);
        return ret;
    }
    /**
     * Get all nodes in the dependency cone (ancestors + node + descendants).
     * @param {number} node
     * @returns {any}
     */
    dependencyCone(node) {
        const ret = wasm.digraph_dependencyCone(this.__wbg_ptr, node);
        return ret;
    }
    /**
     * Get nodes ranked by how many dependents they unblock.
     * Returns array of [node_index, unblock_count] pairs.
     * closed_set is an array of bytes where non-zero means closed.
     * @param {Uint8Array} closed_set
     * @param {number} limit
     * @returns {any}
     */
    unblockRanking(closed_set, limit) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_unblockRanking(this.__wbg_ptr, ptr0, len0, limit);
        return ret;
    }
    /**
     * Get all actionable nodes (nodes with all predecessors in closed_set).
     * closed_set is an array of bytes where non-zero means closed.
     * @param {Uint8Array} closed_set
     * @returns {any}
     */
    actionableNodes(closed_set) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_actionableNodes(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Enumerate all elementary cycles using Johnson's algorithm.
     * Returns JSON: { cycles: number[][], truncated: bool, count: number }
     * @param {number} max_cycles
     * @returns {any}
     */
    enumerateCycles(max_cycles) {
        const ret = wasm.digraph_enumerateCycles(this.__wbg_ptr, max_cycles);
        return ret;
    }
    /**
     * Find k longest paths through the DAG.
     * Returns JSON: { paths: [{nodes, length}], total_nodes, max_length }
     * @param {number} k
     * @returns {any}
     */
    kCriticalPaths(k) {
        const ret = wasm.digraph_kCriticalPaths(this.__wbg_ptr, k);
        return ret;
    }
    /**
     * Compute PageRank with default parameters (damping=0.85, max_iterations=100).
     * @returns {any}
     */
    pagerankDefault() {
        const ret = wasm.digraph_pagerankDefault(this.__wbg_ptr);
        return ret;
    }
    /**
     * TopK Set with default k=5.
     * closed_set is an array of bytes where non-zero means closed.
     * @param {Uint8Array} closed_set
     * @returns {any}
     */
    topkSetDefault(closed_set) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_topkSetDefault(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Topological sort using Kahn's algorithm.
     * Returns node indices in topological order, or null if graph has cycles.
     * @returns {any}
     */
    topologicalSort() {
        const ret = wasm.digraph_topologicalSort(this.__wbg_ptr);
        return ret;
    }
    /**
     * Compute approximate betweenness centrality using sampling.
     * Returns array of scores in node index order.
     * Error: O(1/sqrt(k)) - with k=100, ~10% error in ranking.
     * @param {number} sample_size
     * @returns {any}
     */
    betweennessApprox(sample_size) {
        const ret = wasm.digraph_betweennessApprox(this.__wbg_ptr, sample_size);
        return ret;
    }
    /**
     * Get count of open blockers for a node.
     * closed_set is an array of bytes where non-zero means closed.
     * @param {number} node
     * @param {Uint8Array} closed_set
     * @returns {number}
     */
    openBlockerCount(node, closed_set) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_openBlockerCount(this.__wbg_ptr, node, ptr0, len0);
        return ret >>> 0;
    }
    /**
     * Find articulation points (cut vertices) in the graph.
     * These are nodes whose removal disconnects the graph.
     * Returns array of node indices.
     * @returns {any}
     */
    articulationPoints() {
        const ret = wasm.digraph_articulationPoints(this.__wbg_ptr);
        return ret;
    }
    /**
     * Get nodes on the critical path (those with maximum height).
     * @returns {any}
     */
    criticalPathNodes() {
        const ret = wasm.digraph_criticalPathNodes(this.__wbg_ptr);
        return ret;
    }
    /**
     * Compute eigenvector centrality with default parameters (50 iterations).
     * @returns {any}
     */
    eigenvectorDefault() {
        const ret = wasm.digraph_eigenvectorDefault(this.__wbg_ptr);
        return ret;
    }
    /**
     * Batch what-if: compute impact of closing multiple nodes together.
     * Returns JSON with combined cascade impact.
     * @param {Uint32Array} nodes
     * @param {Uint8Array} closed_set
     * @returns {any}
     */
    whatIfCloseBatch(nodes, closed_set) {
        const ptr0 = passArray32ToWasm0(nodes, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ptr1 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len1 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_whatIfCloseBatch(this.__wbg_ptr, ptr0, len0, ptr1, len1);
        return ret;
    }
    /**
     * Compute coverage set with default limit of 10.
     * @returns {any}
     */
    coverageSetDefault() {
        const ret = wasm.digraph_coverageSetDefault(this.__wbg_ptr);
        return ret;
    }
    /**
     * Get the maximum height (critical path length).
     * @returns {number}
     */
    criticalPathLength() {
        const ret = wasm.digraph_criticalPathLength(this.__wbg_ptr);
        return ret;
    }
    /**
     * Find parallel cut suggestions with default limit of 10.
     * closed_set is an array of bytes where non-zero means closed.
     * @param {Uint8Array} closed_set
     * @returns {any}
     */
    parallelCutDefault(closed_set) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_parallelCutDefault(this.__wbg_ptr, ptr0, len0);
        return ret;
    }
    /**
     * Compute critical path heights (depth in DAG).
     * Returns heights as JSON array, or zeros for cyclic graphs.
     * @returns {any}
     */
    criticalPathHeights() {
        const ret = wasm.digraph_criticalPathHeights(this.__wbg_ptr);
        return ret;
    }
    /**
     * Suggest edges to remove to break cycles.
     * Returns JSON: { suggestions: [{from, to, cycles_broken, collateral, from_id, to_id}], total_cycles, truncated }
     * Suggestions are sorted by cycles_broken desc, then collateral asc.
     * @param {number} limit
     * @param {number} max_cycles_to_enumerate
     * @returns {any}
     */
    cycleBreakSuggestions(limit, max_cycles_to_enumerate) {
        const ret = wasm.digraph_cycleBreakSuggestions(this.__wbg_ptr, limit, max_cycles_to_enumerate);
        return ret;
    }
    /**
     * Quick cycle break suggestions (faster, less precise).
     * Only uses SCC membership without full cycle enumeration.
     * Returns JSON array of { from, to, collateral, from_id, to_id }.
     * @param {number} limit
     * @returns {any}
     */
    quickCycleBreakEdges(limit) {
        const ret = wasm.digraph_quickCycleBreakEdges(this.__wbg_ptr, limit);
        return ret;
    }
    /**
     * Find k longest paths with default k=5.
     * @returns {any}
     */
    kCriticalPathsDefault() {
        const ret = wasm.digraph_kCriticalPathsDefault(this.__wbg_ptr);
        return ret;
    }
    /**
     * Find nodes that increase parallelization when completed.
     * Returns JSON: { items: [{node, parallel_gain, new_actionable}], open_nodes, current_actionable }
     * closed_set is an array of bytes where non-zero means closed.
     * @param {Uint8Array} closed_set
     * @param {number} limit
     * @returns {any}
     */
    parallelCutSuggestions(closed_set, limit) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_parallelCutSuggestions(this.__wbg_ptr, ptr0, len0, limit);
        return ret;
    }
    /**
     * Create an empty graph.
     */
    constructor() {
        const ret = wasm.digraph_new();
        this.__wbg_ptr = ret >>> 0;
        DiGraphFinalization.register(this, this.__wbg_ptr, this);
        return this;
    }
    /**
     * Compute HITS hub and authority scores.
     * Returns JSON object: { hubs: number[], authorities: number[], iterations: number }
     * @param {number} tolerance
     * @param {number} max_iterations
     * @returns {any}
     */
    hits(tolerance, max_iterations) {
        const ret = wasm.digraph_hits(this.__wbg_ptr, tolerance, max_iterations);
        return ret;
    }
    /**
     * Compute k-core numbers for all nodes.
     * Uses undirected view of the graph.
     * Returns array of core numbers in node index order.
     * @returns {any}
     */
    kcore() {
        const ret = wasm.digraph_kcore(this.__wbg_ptr);
        return ret;
    }
    /**
     * Compute slack for each node in the DAG.
     * Slack = critical_path_length - longest_path_through_node.
     * Zero slack means the node is on the critical path.
     * Returns array of slack values, or zeros for cyclic graphs.
     * @returns {any}
     */
    slack() {
        const ret = wasm.digraph_slack(this.__wbg_ptr);
        return ret;
    }
    /**
     * Check if graph is a DAG (directed acyclic graph).
     * @returns {boolean}
     */
    isDag() {
        const ret = wasm.digraph_isDag(this.__wbg_ptr);
        return ret !== 0;
    }
    /**
     * Find bridges (cut edges) in the graph.
     * These are edges whose removal disconnects the graph.
     * Returns array of [from, to] pairs.
     * @returns {any}
     */
    bridges() {
        const ret = wasm.digraph_bridges(this.__wbg_ptr);
        return ret;
    }
    /**
     * Graph density: edges / (nodes * (nodes - 1)).
     * @returns {number}
     */
    density() {
        const ret = wasm.digraph_density(this.__wbg_ptr);
        return ret;
    }
    /**
     * Get node ID by index.
     * @param {number} idx
     * @returns {string | undefined}
     */
    nodeId(idx) {
        const ret = wasm.digraph_nodeId(this.__wbg_ptr, idx);
        let v1;
        if (ret[0] !== 0) {
            v1 = getStringFromWasm0(ret[0], ret[1]).slice();
            wasm.__wbindgen_free(ret[0], ret[1] * 1, 1);
        }
        return v1;
    }
    /**
     * Export graph as JSON snapshot.
     * @returns {string}
     */
    toJson() {
        let deferred1_0;
        let deferred1_1;
        try {
            const ret = wasm.digraph_toJson(this.__wbg_ptr);
            deferred1_0 = ret[0];
            deferred1_1 = ret[1];
            return getStringFromWasm0(ret[0], ret[1]);
        } finally {
            wasm.__wbindgen_free(deferred1_0, deferred1_1, 1);
        }
    }
    /**
     * Add a directed edge from -> to. Idempotent.
     * @param {number} from
     * @param {number} to
     */
    addEdge(from, to) {
        wasm.digraph_addEdge(this.__wbg_ptr, from, to);
    }
    /**
     * Add a node, returns its index. Idempotent - returns existing index if already present.
     * @param {string} id
     * @returns {number}
     */
    addNode(id) {
        const ptr0 = passStringToWasm0(id, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_addNode(this.__wbg_ptr, ptr0, len0);
        return ret >>> 0;
    }
    /**
     * Get direct blockers (predecessors) of a node.
     * These are issues that must be completed before this node can start.
     * @param {number} node
     * @returns {any}
     */
    blockers(node) {
        const ret = wasm.digraph_blockers(this.__wbg_ptr, node);
        return ret;
    }
    /**
     * Get all node IDs as JSON array.
     * @returns {any}
     */
    nodeIds() {
        const ret = wasm.digraph_nodeIds(this.__wbg_ptr);
        return ret;
    }
    /**
     * Get node index by ID.
     * @param {string} id
     * @returns {number | undefined}
     */
    nodeIdx(id) {
        const ptr0 = passStringToWasm0(id, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_nodeIdx(this.__wbg_ptr, ptr0, len0);
        return ret === 0x100000001 ? undefined : ret;
    }
    /**
     * Compute PageRank scores for all nodes.
     * Returns array of scores in node index order.
     * @param {number} damping
     * @param {number} max_iterations
     * @returns {any}
     */
    pagerank(damping, max_iterations) {
        const ret = wasm.digraph_pagerank(this.__wbg_ptr, damping, max_iterations);
        return ret;
    }
    /**
     * Extract a subgraph containing only the specified node indices.
     * Returns a new DiGraph with renumbered indices.
     * @param {Uint32Array} indices
     * @returns {DiGraph}
     */
    subgraph(indices) {
        const ptr0 = passArray32ToWasm0(indices, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_subgraph(this.__wbg_ptr, ptr0, len0);
        return DiGraph.__wrap(ret);
    }
    /**
     * Greedy submodular selection for maximum unlock.
     * Finds k issues that, when completed, maximize total downstream unlocks.
     * Returns JSON: { items: [{node, marginal_gain, unblocked_ids}], total_gain, open_nodes }
     * closed_set is an array of bytes where non-zero means closed.
     * @param {Uint8Array} closed_set
     * @param {number} k
     * @returns {any}
     */
    topkSet(closed_set, k) {
        const ptr0 = passArray8ToWasm0(closed_set, wasm.__wbindgen_malloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_topkSet(this.__wbg_ptr, ptr0, len0, k);
        return ret;
    }
    /**
     * Import graph from JSON snapshot.
     * @param {string} json
     * @returns {DiGraph}
     */
    static fromJson(json) {
        const ptr0 = passStringToWasm0(json, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len0 = WASM_VECTOR_LEN;
        const ret = wasm.digraph_fromJson(ptr0, len0);
        if (ret[2]) {
            throw takeFromExternrefTable0(ret[1]);
        }
        return DiGraph.__wrap(ret[0]);
    }
    /**
     * In-degree of a node (number of dependents).
     * @param {number} node
     * @returns {number}
     */
    inDegree(node) {
        const ret = wasm.digraph_inDegree(this.__wbg_ptr, node);
        return ret >>> 0;
    }
}
if (Symbol.dispose) DiGraph.prototype[Symbol.dispose] = DiGraph.prototype.free;

/**
 * Initialize panic hook for better error messages in browser console.
 */
export function init() {
    wasm.init();
}

/**
 * Get the crate version.
 * @returns {string}
 */
export function version() {
    let deferred1_0;
    let deferred1_1;
    try {
        const ret = wasm.version();
        deferred1_0 = ret[0];
        deferred1_1 = ret[1];
        return getStringFromWasm0(ret[0], ret[1]);
    } finally {
        wasm.__wbindgen_free(deferred1_0, deferred1_1, 1);
    }
}

const EXPECTED_RESPONSE_TYPES = new Set(['basic', 'cors', 'default']);

async function __wbg_load(module, imports) {
    if (typeof Response === 'function' && module instanceof Response) {
        if (typeof WebAssembly.instantiateStreaming === 'function') {
            try {
                return await WebAssembly.instantiateStreaming(module, imports);
            } catch (e) {
                const validResponse = module.ok && EXPECTED_RESPONSE_TYPES.has(module.type);

                if (validResponse && module.headers.get('Content-Type') !== 'application/wasm') {
                    console.warn("`WebAssembly.instantiateStreaming` failed because your server does not serve Wasm with `application/wasm` MIME type. Falling back to `WebAssembly.instantiate` which is slower. Original error:\n", e);

                } else {
                    throw e;
                }
            }
        }

        const bytes = await module.arrayBuffer();
        return await WebAssembly.instantiate(bytes, imports);
    } else {
        const instance = await WebAssembly.instantiate(module, imports);

        if (instance instanceof WebAssembly.Instance) {
            return { instance, module };
        } else {
            return instance;
        }
    }
}

function __wbg_get_imports() {
    const imports = {};
    imports.wbg = {};
    imports.wbg.__wbg_Error_52673b7de5a0ca89 = function(arg0, arg1) {
        const ret = Error(getStringFromWasm0(arg0, arg1));
        return ret;
    };
    imports.wbg.__wbg___wbindgen_is_function_8d400b8b1af978cd = function(arg0) {
        const ret = typeof(arg0) === 'function';
        return ret;
    };
    imports.wbg.__wbg___wbindgen_is_object_ce774f3490692386 = function(arg0) {
        const val = arg0;
        const ret = typeof(val) === 'object' && val !== null;
        return ret;
    };
    imports.wbg.__wbg___wbindgen_is_string_704ef9c8fc131030 = function(arg0) {
        const ret = typeof(arg0) === 'string';
        return ret;
    };
    imports.wbg.__wbg___wbindgen_is_undefined_f6b95eab589e0269 = function(arg0) {
        const ret = arg0 === undefined;
        return ret;
    };
    imports.wbg.__wbg___wbindgen_throw_dd24417ed36fc46e = function(arg0, arg1) {
        throw new Error(getStringFromWasm0(arg0, arg1));
    };
    imports.wbg.__wbg_call_3020136f7a2d6e44 = function() { return handleError(function (arg0, arg1, arg2) {
        const ret = arg0.call(arg1, arg2);
        return ret;
    }, arguments) };
    imports.wbg.__wbg_call_abb4ff46ce38be40 = function() { return handleError(function (arg0, arg1) {
        const ret = arg0.call(arg1);
        return ret;
    }, arguments) };
    imports.wbg.__wbg_crypto_574e78ad8b13b65f = function(arg0) {
        const ret = arg0.crypto;
        return ret;
    };
    imports.wbg.__wbg_error_7534b8e9a36f1ab4 = function(arg0, arg1) {
        let deferred0_0;
        let deferred0_1;
        try {
            deferred0_0 = arg0;
            deferred0_1 = arg1;
            console.error(getStringFromWasm0(arg0, arg1));
        } finally {
            wasm.__wbindgen_free(deferred0_0, deferred0_1, 1);
        }
    };
    imports.wbg.__wbg_getRandomValues_b8f5dbd5f3995a9e = function() { return handleError(function (arg0, arg1) {
        arg0.getRandomValues(arg1);
    }, arguments) };
    imports.wbg.__wbg_length_22ac23eaec9d8053 = function(arg0) {
        const ret = arg0.length;
        return ret;
    };
    imports.wbg.__wbg_msCrypto_a61aeb35a24c1329 = function(arg0) {
        const ret = arg0.msCrypto;
        return ret;
    };
    imports.wbg.__wbg_new_1ba21ce319a06297 = function() {
        const ret = new Object();
        return ret;
    };
    imports.wbg.__wbg_new_25f239778d6112b9 = function() {
        const ret = new Array();
        return ret;
    };
    imports.wbg.__wbg_new_8a6f238a6ece86ea = function() {
        const ret = new Error();
        return ret;
    };
    imports.wbg.__wbg_new_no_args_cb138f77cf6151ee = function(arg0, arg1) {
        const ret = new Function(getStringFromWasm0(arg0, arg1));
        return ret;
    };
    imports.wbg.__wbg_new_with_length_aa5eaf41d35235e5 = function(arg0) {
        const ret = new Uint8Array(arg0 >>> 0);
        return ret;
    };
    imports.wbg.__wbg_node_905d3e251edff8a2 = function(arg0) {
        const ret = arg0.node;
        return ret;
    };
    imports.wbg.__wbg_process_dc0fbacc7c1c06f7 = function(arg0) {
        const ret = arg0.process;
        return ret;
    };
    imports.wbg.__wbg_prototypesetcall_dfe9b766cdc1f1fd = function(arg0, arg1, arg2) {
        Uint8Array.prototype.set.call(getArrayU8FromWasm0(arg0, arg1), arg2);
    };
    imports.wbg.__wbg_randomFillSync_ac0988aba3254290 = function() { return handleError(function (arg0, arg1) {
        arg0.randomFillSync(arg1);
    }, arguments) };
    imports.wbg.__wbg_require_60cc747a6bc5215a = function() { return handleError(function () {
        const ret = module.require;
        return ret;
    }, arguments) };
    imports.wbg.__wbg_set_3f1d0b984ed272ed = function(arg0, arg1, arg2) {
        arg0[arg1] = arg2;
    };
    imports.wbg.__wbg_set_7df433eea03a5c14 = function(arg0, arg1, arg2) {
        arg0[arg1 >>> 0] = arg2;
    };
    imports.wbg.__wbg_stack_0ed75d68575b0f3c = function(arg0, arg1) {
        const ret = arg1.stack;
        const ptr1 = passStringToWasm0(ret, wasm.__wbindgen_malloc, wasm.__wbindgen_realloc);
        const len1 = WASM_VECTOR_LEN;
        getDataViewMemory0().setInt32(arg0 + 4 * 1, len1, true);
        getDataViewMemory0().setInt32(arg0 + 4 * 0, ptr1, true);
    };
    imports.wbg.__wbg_static_accessor_GLOBAL_769e6b65d6557335 = function() {
        const ret = typeof global === 'undefined' ? null : global;
        return isLikeNone(ret) ? 0 : addToExternrefTable0(ret);
    };
    imports.wbg.__wbg_static_accessor_GLOBAL_THIS_60cf02db4de8e1c1 = function() {
        const ret = typeof globalThis === 'undefined' ? null : globalThis;
        return isLikeNone(ret) ? 0 : addToExternrefTable0(ret);
    };
    imports.wbg.__wbg_static_accessor_SELF_08f5a74c69739274 = function() {
        const ret = typeof self === 'undefined' ? null : self;
        return isLikeNone(ret) ? 0 : addToExternrefTable0(ret);
    };
    imports.wbg.__wbg_static_accessor_WINDOW_a8924b26aa92d024 = function() {
        const ret = typeof window === 'undefined' ? null : window;
        return isLikeNone(ret) ? 0 : addToExternrefTable0(ret);
    };
    imports.wbg.__wbg_subarray_845f2f5bce7d061a = function(arg0, arg1, arg2) {
        const ret = arg0.subarray(arg1 >>> 0, arg2 >>> 0);
        return ret;
    };
    imports.wbg.__wbg_versions_c01dfd4722a88165 = function(arg0) {
        const ret = arg0.versions;
        return ret;
    };
    imports.wbg.__wbindgen_cast_2241b6af4c4b2941 = function(arg0, arg1) {
        // Cast intrinsic for `Ref(String) -> Externref`.
        const ret = getStringFromWasm0(arg0, arg1);
        return ret;
    };
    imports.wbg.__wbindgen_cast_4625c577ab2ec9ee = function(arg0) {
        // Cast intrinsic for `U64 -> Externref`.
        const ret = BigInt.asUintN(64, arg0);
        return ret;
    };
    imports.wbg.__wbindgen_cast_cb9088102bce6b30 = function(arg0, arg1) {
        // Cast intrinsic for `Ref(Slice(U8)) -> NamedExternref("Uint8Array")`.
        const ret = getArrayU8FromWasm0(arg0, arg1);
        return ret;
    };
    imports.wbg.__wbindgen_cast_d6cd19b81560fd6e = function(arg0) {
        // Cast intrinsic for `F64 -> Externref`.
        const ret = arg0;
        return ret;
    };
    imports.wbg.__wbindgen_init_externref_table = function() {
        const table = wasm.__wbindgen_externrefs;
        const offset = table.grow(4);
        table.set(0, undefined);
        table.set(offset + 0, undefined);
        table.set(offset + 1, null);
        table.set(offset + 2, true);
        table.set(offset + 3, false);
    };

    return imports;
}

function __wbg_finalize_init(instance, module) {
    wasm = instance.exports;
    __wbg_init.__wbindgen_wasm_module = module;
    cachedDataViewMemory0 = null;
    cachedUint32ArrayMemory0 = null;
    cachedUint8ArrayMemory0 = null;


    wasm.__wbindgen_start();
    return wasm;
}

function initSync(module) {
    if (wasm !== undefined) return wasm;


    if (typeof module !== 'undefined') {
        if (Object.getPrototypeOf(module) === Object.prototype) {
            ({module} = module)
        } else {
            console.warn('using deprecated parameters for `initSync()`; pass a single object instead')
        }
    }

    const imports = __wbg_get_imports();
    if (!(module instanceof WebAssembly.Module)) {
        module = new WebAssembly.Module(module);
    }
    const instance = new WebAssembly.Instance(module, imports);
    return __wbg_finalize_init(instance, module);
}

async function __wbg_init(module_or_path) {
    if (wasm !== undefined) return wasm;


    if (typeof module_or_path !== 'undefined') {
        if (Object.getPrototypeOf(module_or_path) === Object.prototype) {
            ({module_or_path} = module_or_path)
        } else {
            console.warn('using deprecated parameters for the initialization function; pass a single object instead')
        }
    }

    if (typeof module_or_path === 'undefined') {
        module_or_path = new URL('bv_graph_bg.wasm', import.meta.url);
    }
    const imports = __wbg_get_imports();

    if (typeof module_or_path === 'string' || (typeof Request === 'function' && module_or_path instanceof Request) || (typeof URL === 'function' && module_or_path instanceof URL)) {
        module_or_path = fetch(module_or_path);
    }

    const { instance, module } = await __wbg_load(await module_or_path, imports);

    return __wbg_finalize_init(instance, module);
}

export { initSync };
export default __wbg_init;
