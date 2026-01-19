# sgrep - Smart Code & Conversation Search Skill

## Purpose

Use `sgrep` for semantic and hybrid search across **code** and **agent conversations** when you need to find content by **intent** rather than exact text patterns.

## When to Use This Skill

### Code Search
- Searching for concepts: "error handling", "authentication", "caching logic"
- Searching with specific terms + context: use `--hybrid` for "JWT validation", "OAuth2 token"
- Exploring unfamiliar codebases
- When ripgrep patterns keep missing relevant code
- Finding implementations of features described in natural language

### Conversation Search
- Finding past discussions with **Claude Code**, **Codex CLI**, or **Cursor**
- Recalling how a similar problem was solved before
- Building context from previous sessions for new tasks
- Searching across all coding agent interactions

## Setup (One-time)

```bash
# Install llama.cpp
brew install llama.cpp

# Download embedding model
sgrep setup

# Index the codebase
sgrep index .
```

## Commands

| Command | Purpose |
|---------|---------|
| `sgrep "query"` | Semantic search (understands intent) |
| `sgrep --hybrid "query"` | Hybrid search (semantic + BM25 term matching) |
| `sgrep -c "query"` | Search with code context |
| `sgrep --json "query"` | JSON output for parsing |
| `sgrep -q "query"` | Quiet mode (paths only) |
| `sgrep -t "query"` | Include test files |
| `sgrep index .` | Index current directory |
| `sgrep server status` | Check embedding server |

## Semantic vs Hybrid Search

| Mode | Best For | Example Query |
|------|----------|---------------|
| Semantic (default) | Conceptual questions | "how does auth work" |
| Hybrid (`--hybrid`) | Queries with specific terms | "JWT token validation" |

**Use `--hybrid`** when your query contains:
- Function/API names: `--hybrid "parseAST"`
- Technical terms: `--hybrid "OAuth2 refresh token"`
- Specific keywords that should match exactly

**Use semantic (default)** for:
- Conceptual questions: "how is caching implemented"
- Intent-based search: "error handling logic"

## Search Strategy

### The Search Hierarchy

1. **sgrep** → Find relevant files/functions by semantic intent
2. **sgrep --hybrid** → Find code matching intent + specific terms
3. **ast-grep (sg)** → Match structural patterns in those files
4. **ripgrep (rg)** → Exact text for specific symbols

### Example Workflow

```bash
# Step 1: Semantic discovery
sgrep "rate limiting implementation"
# → api/ratelimit.go:20-80

# Step 1b: Or use hybrid for specific terms
sgrep --hybrid "RateLimiter middleware"
# → api/middleware.go:45-90

# Step 2: Structural patterns
sg -p 'rateLimiter.Check($ctx, $key)'

# Step 3: Exact search
rg "RATE_LIMIT_MAX"
```

## Output Interpretation

```bash
$ sgrep "authentication"
auth/middleware.go:45-67      # file:startLine-endLine
auth/jwt.go:12-38
handlers/login.go:89-112
```

Lower scores = more relevant (L2 distance for semantic, hybrid score for `--hybrid`).

## Hybrid Search Tuning

```bash
# Default weights: 60% semantic, 40% BM25
sgrep --hybrid "query"

# More weight on exact term matching
sgrep --hybrid --semantic-weight 0.4 --bm25-weight 0.6 "parseConfig"

# More weight on semantic understanding
sgrep --hybrid --semantic-weight 0.8 --bm25-weight 0.2 "configuration loading"
```

## Conversation Search

Search across conversations with Claude Code, Codex CLI, and Cursor.

### Commands

| Command | Purpose |
|---------|---------|
| `sgrep conv "query"` | Semantic search across all agents |
| `sgrep conv "query" --agent claude` | Filter by agent |
| `sgrep conv "query" --since 7d` | Filter by time |
| `sgrep conv "query" --project myapp` | Filter by project |
| `sgrep conv "query" --hybrid` | Semantic + keyword search |
| `sgrep conv view <session-id>` | View full conversation |
| `sgrep conv context <session-id>` | Extract context for new session |
| `sgrep conv index` | Index/update conversations |
| `sgrep conv status` | Check index status |

### Example Workflow

```bash
# Find how you fixed a similar bug before
sgrep conv "race condition fix" --since 1m

# Search Claude Code sessions for a specific project
sgrep conv "database migration" --agent claude --project myapp

# Get context from a past session to inject into current work
sgrep conv context abc123
```

## Tips

- Use natural language queries: "how does the cache invalidation work"
- Use `--hybrid` when searching for specific function names or technical terms
- Combine with `-c` flag to see code snippets
- Use `--json` when parsing results programmatically
- Server auto-starts; use `sgrep server stop` to free resources when done
- Run `sgrep conv index` periodically to keep conversation index updated

## Troubleshooting

```bash
# Check server status
sgrep server status

# Re-download model if corrupted
rm -rf ~/.sgrep/models
sgrep setup

# Re-index if results seem stale
sgrep clear
sgrep index .
```
