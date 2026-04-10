# bv-graph-wasm

High-performance graph algorithms for the bv static viewer, compiled to WebAssembly.

## Prerequisites

```bash
rustup --version          # Rust toolchain
cargo --version           # Rust package manager
wasm-pack --version       # WASM build tool (>= 0.12)
```

Install wasm-pack if needed:
```bash
cargo install wasm-pack
```

## Building

```bash
# Development build (faster, larger)
make build

# Release build (optimized, smaller)
make build-release

# Run tests
make test
```

## Output

After building, the `pkg/` directory contains:
- `bv_graph_wasm.js` - JavaScript bindings
- `bv_graph_wasm_bg.wasm` - WebAssembly binary
- `bv_graph_wasm.d.ts` - TypeScript definitions

## Usage

```javascript
import init, { DiGraph, version } from './pkg/bv_graph_wasm.js';

async function main() {
    await init();

    console.log('Version:', version());

    const graph = new DiGraph();
    const a = graph.addNode('bv-1');
    const b = graph.addNode('bv-2');
    graph.addEdge(a, b);

    console.log('Nodes:', graph.nodeCount());
    console.log('Edges:', graph.edgeCount());
    console.log('Density:', graph.density());

    // Export/import
    const json = graph.toJson();
    const graph2 = DiGraph.fromJson(json);

    // Don't forget to free when done
    graph.free();
    graph2.free();
}

main();
```

## API

### DiGraph

| Method | Description |
|--------|-------------|
| `new()` | Create empty graph |
| `withCapacity(n, e)` | Create with pre-allocated capacity |
| `addNode(id)` | Add node, returns index (idempotent) |
| `addEdge(from, to)` | Add directed edge (idempotent) |
| `nodeCount()` | Number of nodes |
| `edgeCount()` | Number of edges |
| `density()` | Graph density |
| `nodeId(idx)` | Get node ID by index |
| `nodeIdx(id)` | Get node index by ID |
| `nodeIds()` | All node IDs as array |
| `outDegree(node)` | Out-degree of node |
| `inDegree(node)` | In-degree of node |
| `successors(node)` | Get successor indices |
| `predecessors(node)` | Get predecessor indices |
| `toJson()` | Export as JSON |
| `fromJson(json)` | Import from JSON |
| `free()` | Release memory |

## Size

### Current Measurements

| Component | Raw | Gzipped | Budget |
|-----------|-----|---------|--------|
| WASM binary | ~213KB | ~94KB | <80KB |
| JS glue | ~36KB | ~7KB | <25KB |
| **Total** | ~249KB | ~101KB | **<120KB** |

### Size Optimization

The build pipeline applies multiple optimizations:

1. **Cargo.toml profile settings**:
   - `opt-level = "s"` - Optimize for size
   - `lto = true` - Link-time optimization
   - `codegen-units = 1` - Better optimization
   - `panic = "abort"` - Remove panic unwinding

2. **wasm-pack optimization**:
   - Built-in wasm-opt pass during `--release` builds

3. **Optional: Additional wasm-opt** (if installed):
   ```bash
   # Install binaryen for standalone wasm-opt
   brew install binaryen  # macOS
   apt install binaryen   # Ubuntu

   # Run with extra optimization passes
   wasm-opt -Os -o pkg/bv_graph_wasm_bg.wasm pkg/bv_graph_wasm_bg.wasm
   ```

### Checking Size

```bash
make size  # Shows raw and gzipped sizes
```

### Feature Flags

The crate supports feature flags for optional algorithms:

| Feature | Description | Default |
|---------|-------------|---------|
| `core` | Core algorithms (pagerank, betweenness, cycles, critical path) | Yes |
| `eigenvector` | Eigenvector centrality | No |
| `kcore` | K-core decomposition | No |
| `slack` | Slack/float calculations | No |
| `hits` | HITS algorithm | No |
| `reachability` | Reachability queries | No |
| `full` | All algorithms | No |

Build with specific features:
```bash
wasm-pack build --target web --release -- --features "core,eigenvector"
```

## License

MIT
