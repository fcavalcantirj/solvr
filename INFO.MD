/gsd:plan-phase 13                                                                                                                                             
                                                                                                                                                                 
  Context: v1.3 Quorum Merge + Live Search. Phase 13 = Database Foundation.                                                                                      
                                                                                                                                                                 
  Create migrations 000073-000075 for Solvr's PostgreSQL:
  - 000073: rooms table (from Quorum schema.sql — slug, display_name, description, tags, is_private, owner_id FK to users, token_hash, expires_at)               
  - 000074: agent_presence table (room_id FK, agent_name, card_json JSONB, ttl_seconds default 600, last_seen)                                    
  - 000075: messages table (room_id FK, agent_name, content TEXT, author_type, author_id for human comments) + room_comments table (room_id FK, user_id FK,      
  content, created_at)                                                                                                                                           
                                                                                                                                                                 
  DO NOT migrate Quorum's users or refresh_tokens tables — Solvr already has those.                                                                              
  Quorum source at /Users/fcavalcanti/dev/quorum/relay/schema.sql for reference.                                                                                 
  Research files at .planning/research/ — read PITFALLS.md before starting (table conflicts, format conversion from goose to golang-migrate).
  Full context in .planning/STATE.md accumulated context section.                                                                                                
                                                                                                                                                                 
  ---                                                                                                                                                            
  Everything is saved:                                                                                                                                           
  - .planning/STATE.md — full session context, decisions, blockers, Quorum codebase reference                                                                    
  - .planning/research/ — 5 files (STACK, FEATURES, ARCHITECTURE, PITFALLS, SUMMARY)         
  - .planning/ROADMAP.md — 5 phases, 22 requirements mapped                                                                                                      
  - .planning/REQUIREMENTS.md — 22 requirements with traceability                                                                                                
  - Memory files — usage analysis + merge context for future sessions                                                                                            
  - Quorum room — 42 messages at solvr-usage-analysys