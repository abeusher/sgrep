# sgrep - Smart Code & Conversation Search

Use `sgrep` for semantic and hybrid search across **code** and **agent conversations**. It understands intent, not just exact strings.

## When to Use

### Code Search
- Finding code by **concept**: "error handling", "authentication logic", "rate limiting"
- Searching for **specific terms** with semantic context: use `--hybrid`
- Exploring unfamiliar codebases
- When ripgrep patterns keep missing relevant code

### Conversation Search
- Finding past discussions with **Claude Code**, **Codex CLI**, or **Cursor**
- Recalling how you solved a similar problem before
- Building context from previous sessions for new tasks
- Searching across all your coding agent interactions

## Quick Reference

```bash
# First time only
sgrep setup

# Index current directory
sgrep index .

# Semantic search (understands intent)
sgrep "database connection pooling"
sgrep "how are errors handled"

# Hybrid search (semantic + exact term matching via BM25)
sgrep --hybrid "JWT validation"
sgrep --hybrid "authentication middleware"

# Tune hybrid weights (default: 60% semantic, 40% BM25)
sgrep --hybrid --semantic-weight 0.5 --bm25-weight 0.5 "error handler"

# With code context
sgrep -c "authentication middleware"

# JSON output (for parsing)
sgrep --json "error handling"

# Quiet mode (paths only)
sgrep -q "logging"

# Include test files
sgrep -t "mock database"
```

## Semantic vs Hybrid

| Mode | Best For | Example |
|------|----------|---------|
| Semantic (default) | Conceptual queries | "how does auth work" |
| Hybrid (`--hybrid`) | Queries with specific terms | "JWT token validation" |

**Use hybrid when** your query contains exact technical terms (function names, APIs, specific keywords) that should be matched literally alongside semantic understanding.

## Search Hierarchy

1. **sgrep** → Find files/functions by intent (semantic) or intent + terms (hybrid)
2. **ast-grep** → Match structural patterns
3. **ripgrep** → Exact text search

## Example Workflow

```bash
# Find authentication code semantically
sgrep "user authentication flow"
# → auth/handler.go:45-80

# Or use hybrid for specific term matching
sgrep --hybrid "OAuth2 token refresh"
# → auth/oauth.go:120-150

# Then use ast-grep for structural patterns
sg -p 'if err != nil { return $_ }'

# Then ripgrep for specific symbols
rg "JWT_SECRET"
```

## Flags

| Flag | Description |
|------|-------------|
| `-n, --limit N` | Max results (default: 10) |
| `-c, --context` | Show code context |
| `--json` | JSON output |
| `-q, --quiet` | Paths only |
| `-t, --include-tests` | Include test files |
| `--hybrid` | Enable hybrid search (semantic + BM25) |
| `--semantic-weight` | Weight for semantic (default: 0.6) |
| `--bm25-weight` | Weight for BM25 (default: 0.4) |
| `--threshold` | Distance threshold (default: 1.5) |

## Conversation Search

Search across your conversations with Claude Code, Codex CLI, and Cursor.

```bash
# Index conversations (run periodically to update)
sgrep conv index

# Semantic search across all agents
sgrep conv "authentication flow"
sgrep conv "how did I fix that race condition"

# Filter by agent
sgrep conv "database migration" --agent claude
sgrep conv "API design" --agent cursor

# Filter by time
sgrep conv "bug fix" --since 7d
sgrep conv "refactoring" --since 1m

# Filter by project
sgrep conv "testing strategy" --project payment-service

# Hybrid search (semantic + keyword)
sgrep conv "JWT refresh_token" --hybrid

# View full conversation
sgrep conv view <session-id>

# Extract context for new session
sgrep conv context <session-id>

# Check index status
sgrep conv status
```

### Conversation Search Flags

| Flag | Description |
|------|-------------|
| `-a, --agent` | Filter: claude, codex, cursor, all |
| `--since` | Time filter: 1h, 7d, 2w, 1m, 1y |
| `-p, --project` | Filter by project name/path |
| `--hybrid` | Semantic + keyword search |
| `-n, --limit` | Max results (default: 10) |
| `-v, --verbose` | Show full turn content |
| `--json` | JSON output |

## Server Management

The embedding server auto-starts. Manual control:

```bash
sgrep server status   # Check status
sgrep server stop     # Stop server
```
