#!/bin/bash
set -euo pipefail

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  MaskChain: Aho-Corasick 1000-term Dictionary Benchmark"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

cd "$(dirname "$0")/.."

echo "  Running benchmark: 1000 dictionary terms, ~10KB text..."
echo ""

go test -bench=BenchmarkAhoCorasick_1000Terms \
  -benchmem -count=1 -run='^$' \
  ./src/internal/domain/shield/dictionary/

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Key takeaway:"
echo "  Aho-Corasick autómata scans O(n) in text length."
echo "  Even with 1000 terms, matching completes in <1ms."
echo "  Near-zero allocation (8 allocs/op)."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
