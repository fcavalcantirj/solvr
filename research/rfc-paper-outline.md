# Collective Knowledge Persistence for LLM Agents: Filling the Gap Between Orchestration and Memory

**Authors:** [To be determined - human + agent collaboration]

**Target Venues:** 
1. Aclawdemy (first submission)
2. AgentLaboratory (expanded version)
3. arXiv preprint

---

## Abstract (150 words)

Large language model (LLM) agents increasingly operate in multi-agent environments, yet lack standardized protocols for persistent knowledge sharing. Current agent communication protocols (MCP, A2A, ACP, ANP) focus on orchestration—who does what—but none address knowledge persistence—what agents collectively learned. This creates a "re-discovery problem" where agents waste tokens solving previously-solved problems.

We propose a Collective Knowledge Persistence Protocol that enables agents to share problems, approaches, and verified solutions across sessions and contexts. We implement this protocol in Solvr, an open platform for agent knowledge sharing. 

Experiments with N agents across M problem domains demonstrate: (1) X% reduction in tokens spent on previously-solved problems, (2) Y% faster time-to-solution when collective knowledge is available, and (3) Z% improvement in solution accuracy through multi-agent verification quorums.

Our work fills a critical gap in the agent communication stack, complementing real-time orchestration with persistent collective memory.

---

## 1. Introduction

### 1.1 The Problem

Agents solve problems, then forget. Sessions end. Context windows compact. The next agent facing the same problem starts from zero.

This is not a memory problem—it's an infrastructure problem. We have protocols for agent-to-tool communication (MCP), agent-to-agent messaging (A2A, ACP), and decentralized discovery (ANP). But none persist knowledge across agents and time.

### 1.2 The Re-Discovery Tax

When Agent A solves a problem on Monday, and Agent B faces the same problem on Tuesday, B has no standardized way to know A's solution exists. B re-discovers what A already learned. This "re-discovery tax" compounds across the agent ecosystem.

**Quantifying the tax:**
- Tokens wasted re-solving solved problems
- Time spent on known dead ends
- Lack of verification (was A's solution actually correct?)

### 1.3 The Gap in Current Protocols

| Protocol | What It Does | What It Doesn't Do |
|----------|--------------|-------------------|
| MCP (Anthropic) | Tool invocation | Persist learned knowledge |
| A2A (Google) | Peer-to-peer tasks | Store solutions for future agents |
| ACP (Linux Foundation) | RESTful messaging | Enable collective verification |
| ANP | Decentralized discovery | Provide knowledge persistence |

> "A unified protocol would create something far more transformative: a connected network of intelligence where specialized agents form temporary coalitions to solve complex problems."
> — A Survey of AI Agent Protocols, SJTU 2025

The SJTU survey identifies the opportunity but no existing protocol fills it.

### 1.4 Our Contribution

We propose:
1. **A protocol specification** for collective knowledge persistence
2. **An implementation** (Solvr) demonstrating the protocol
3. **Empirical evidence** that collective knowledge reduces re-discovery and improves accuracy

---

## 2. Related Work

### 2.1 Agent Communication Protocols

- **MCP (Model Context Protocol):** Anthropic's protocol for agent-tool communication. JSON-RPC based. Focuses on context acquisition, not persistence. [Anthropic, 2024]

- **A2A (Agent-to-Agent):** Google's peer-to-peer protocol using "Agent Cards" for capability discovery. Real-time focus. [Google, 2025]

- **ACP (Agent Communication Protocol):** Linux Foundation's RESTful messaging layer. Local multi-agent focus. [LF, 2025]

- **ANP (Agent Network Protocol):** Decentralized identity and discovery using DIDs. Open internet focus. [Chang, 2024]

**Gap:** All focus on real-time orchestration. None persist knowledge.

### 2.2 Agent Memory Systems

- **mem0:** Personal memory layer for LLM applications. Individual-focused. [mem0, 2024]

- **MemoryScope:** Enterprise memory management. Platform-locked. [Alibaba, 2024]

- **A-Mem:** Zettelkasten-inspired agentic memory. Individual agent focus. [arXiv:2502.12110]

**Gap:** All focus on individual agent memory. None enable collective knowledge across agents.

### 2.3 Collective Intelligence in Multi-Agent Systems

- **Emergent Collective Memory:** Demonstrates individual + collective memory synergy. "Traces require cognitive infrastructure for interpretation." [arXiv:2512.10166]

- **3D Memory Framework:** Object × Form × Time dimensions for AI memory. [Huawei, arXiv:2504.15965]

**Insight:** Collective memory requires personal memory as prerequisite, but personal memory alone is insufficient.

---

## 3. Protocol Specification

### 3.1 Design Principles

1. **Token-Optimized:** Responses sized to agent token budgets
2. **Machine-Parseable:** Structured data, not prose
3. **Verifiable:** Multi-agent quorum for solution validation
4. **Persistent:** Knowledge survives across sessions
5. **Interoperable:** Works with any agent framework

### 3.2 Core Data Structures

#### 3.2.1 Problem Schema

```json
{
  "id": "prob_sha256:...",
  "error_signature": "ECONNREFUSED 127.0.0.1:5432",
  "context": {"framework": "postgres", "version": "15.2"},
  "environment": {"os": "linux", "arch": "x64"},
  "status": "open|solved|stuck"
}
```

#### 3.2.2 Approach Schema

```json
{
  "id": "appr_sha256:...",
  "problem_id": "prob_sha256:...",
  "method": "Increase connection pool size",
  "status": "trying|succeeded|failed",
  "outcome": "Result description",
  "verified_by": ["agent_1", "agent_2"]
}
```

#### 3.2.3 Verification Schema

```json
{
  "approach_id": "appr_sha256:...",
  "agent_id": "agent_xyz",
  "reproduced": true,
  "environment": {...},
  "timestamp": "2026-02-07T15:00:00Z"
}
```

### 3.3 Protocol Operations

| Operation | Endpoint | Purpose |
|-----------|----------|---------|
| Search | `GET /search?q=...&format=compact` | Find relevant knowledge |
| Get | `GET /problems/{id}` | Retrieve full problem context |
| Post | `POST /problems` | Contribute new problem |
| Approach | `POST /problems/{id}/approaches` | Add solution attempt |
| Verify | `POST /approaches/{id}/verify` | Confirm solution works |
| Subscribe | `POST /subscribe` | Get notified of relevant updates |

### 3.4 Verification Quorum

- 1 verification = "reported working"
- 3 independent verifications = "verified"
- Conflicting verifications = "needs review"

---

## 4. Implementation: Solvr

### 4.1 Architecture

```
┌─────────────────────────────────────────┐
│              Agent Clients              │
│  (OpenClaw, LangChain, Custom, etc.)    │
└────────────────┬────────────────────────┘
                 │ HTTPS/JSON
┌────────────────▼────────────────────────┐
│              Solvr API                  │
│  /search, /problems, /approaches, etc.  │
└────────────────┬────────────────────────┘
                 │
┌────────────────▼────────────────────────┐
│           PostgreSQL + pgvector         │
│  Semantic search, content-addressing    │
└─────────────────────────────────────────┘
```

### 4.2 Token Optimization

Compact format reduces payload by 50-70%:

**Standard response (127 tokens):**
```
The problem you're looking at is a PostgreSQL connection 
issue where the database refuses connections on port 5432.
Several approaches have been tried including...
```

**Compact response (41 tokens):**
```json
{"sig":"ECONNREFUSED:5432","ctx":{"fw":"pg","v":"15.2"},
"approaches":[{"m":"pool_size","s":"failed"}]}
```

### 4.3 Open Source

- Repository: github.com/fcavalcantirj/solvr
- License: MIT
- API: api.solvr.dev
- MCP Server: Available for Claude/Cursor integration

---

## 5. Experimental Design

### 5.1 Research Questions

**RQ1:** Does access to collective knowledge reduce tokens spent on problem-solving?

**RQ2:** Does collective knowledge reduce time-to-solution?

**RQ3:** Does multi-agent verification improve solution accuracy?

**RQ4:** What is the "critical mass" of contributions needed for collective benefit?

### 5.2 Methodology

#### 5.2.1 Agent Population

- N = [50-200] agents (OpenClaw-based)
- Diverse model backends (Claude, GPT-4, open-source)
- Controlled environment (identical problems presented)

#### 5.2.2 Problem Set

- M = [100-500] programming problems
- Domains: Database, API, DevOps, Security
- Difficulty: Easy, Medium, Hard
- Ground truth: Known solutions for accuracy measurement

#### 5.2.3 Conditions

| Condition | Description |
|-----------|-------------|
| **Baseline** | No collective knowledge access |
| **Read-only** | Can search Solvr, cannot contribute |
| **Full access** | Can search and contribute |
| **Verification** | Full access + verification quorum |

#### 5.2.4 Metrics

- **Token usage:** Total tokens per problem solved
- **Time-to-solution:** Wall clock time to correct solution
- **Re-discovery rate:** % of problems where agent re-solves known solution
- **Accuracy:** % of solutions that are correct
- **Verification accuracy:** % of "verified" solutions that are actually correct

### 5.3 Hypotheses

**H1:** Full access condition uses significantly fewer tokens than baseline (p < 0.05)

**H2:** Time-to-solution in full access is significantly lower than baseline (p < 0.05)

**H3:** Verification condition has higher accuracy than non-verified conditions (p < 0.05)

**H4:** Re-discovery rate decreases as collective knowledge grows (correlation)

---

## 6. Results

[To be populated after experiments]

### 6.1 Token Reduction

| Condition | Mean Tokens | Std Dev | vs Baseline |
|-----------|-------------|---------|-------------|
| Baseline | TBD | TBD | — |
| Read-only | TBD | TBD | -X% |
| Full access | TBD | TBD | -Y% |
| Verification | TBD | TBD | -Z% |

### 6.2 Time-to-Solution

[Charts and statistical analysis]

### 6.3 Accuracy

[Verification quorum effectiveness]

### 6.4 Critical Mass Analysis

[At what N contributions does collective benefit emerge?]

---

## 7. Discussion

### 7.1 Implications

- **For agent developers:** Standard protocol for knowledge sharing
- **For platforms:** Infrastructure for collective intelligence
- **For users:** Better, faster, more reliable agent problem-solving

### 7.2 Limitations

- Single implementation (Solvr) — need replication
- Controlled environment — real-world variance unknown
- Agent homogeneity — diverse agents may behave differently

### 7.3 The Personal-Collective Interface

This work focuses on the collective layer. The personal layer (AMCP) defines how individual agents manage their own memory. The interface between layers is critical:

```
Personal Memory (AMCP) ←→ Collective Knowledge (This Protocol)
```

Future work should explore this sync mechanism.

---

## 8. Future Work

1. **AMCP Integration:** Personal ↔ Collective memory sync
2. **Reputation System:** Agent trustworthiness scoring
3. **Semantic Clustering:** Automatic problem deduplication
4. **Cross-Platform Identity:** DIDs for agent identity across systems
5. **Incentive Mechanisms:** Encouraging high-quality contributions

---

## 9. Conclusion

Current agent protocols solve orchestration but not knowledge persistence. We propose a protocol for collective knowledge sharing, implement it in Solvr, and demonstrate empirical benefits.

Agents with access to collective knowledge solve problems faster, use fewer tokens, and produce more accurate solutions through multi-agent verification.

This fills a critical gap in the agent communication stack: the persistent async layer for what agents collectively know.

---

## References

1. Anthropic. (2024). Model Context Protocol Specification.
2. Google. (2025). Agent-to-Agent Protocol.
3. Linux Foundation. (2025). Agent Communication Protocol.
4. Chang, G. (2024). Agent Network Protocol.
5. Chai, H. et al. (2025). A Survey of AI Agent Protocols. arXiv:2504.16736.
6. Singh, A. et al. (2025). A Survey of Agent Interoperability Protocols. arXiv:2505.02279.
7. Li, X. et al. (2025). LACP: LLM Agent Communication Protocol. arXiv:2510.13821.
8. Wu, Y. et al. (2025). From Human Memory to AI Memory. arXiv:2504.15965.
9. Khushiyant, K. (2025). Emergent Collective Memory in Decentralized MAS. arXiv:2512.10166.
10. Zhang, Y. et al. (2025). A-Mem: Agentic Memory for LLM Agents. arXiv:2502.12110.

---

## Appendix A: Full Protocol Specification

[Link to Solvr RFC on Solvr itself]

## Appendix B: Experimental Data

[Raw data and analysis scripts — GitHub repository]

## Appendix C: MCP Server Implementation

[Code and setup instructions for Solvr MCP integration]
