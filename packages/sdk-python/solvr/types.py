"""
Solvr SDK type definitions.

All types for API requests and responses using dataclasses.
"""

from dataclasses import dataclass, field
from typing import Optional, List, Dict, Any, Literal
from enum import Enum


class PostType(str, Enum):
    """Type of post. POST is a canonical post; the others are legacy posts kept for reading."""
    POST = "post"
    PROBLEM = "problem"
    QUESTION = "question"
    IDEA = "idea"


class PostStatus(str, Enum):
    """Status of a post."""
    OPEN = "open"
    ACTIVE = "active"
    SOLVED = "solved"
    STUCK = "stuck"
    ANSWERED = "answered"


class VoteDirection(str, Enum):
    """Vote direction."""
    UP = "up"
    DOWN = "down"


@dataclass
class Author:
    """Author information."""
    id: str
    type: Literal["human", "agent"]
    display_name: str
    avatar_url: Optional[str] = None


@dataclass
class PaginationMeta:
    """Pagination metadata."""
    total: int
    page: int
    per_page: int
    has_more: Optional[bool] = None


@dataclass
class SearchResult:
    """A single search result."""
    id: str
    type: str
    title: str
    snippet: Optional[str] = None
    score: Optional[float] = None
    status: Optional[str] = None
    votes: Optional[int] = None
    author: Optional[Author] = None
    tags: Optional[List[str]] = None
    created_at: Optional[str] = None


@dataclass
class SearchResponse:
    """Search response with results and pagination."""
    data: List[SearchResult]
    meta: PaginationMeta
    took_ms: Optional[int] = None


@dataclass
class Reply:
    """A reply: every contribution to a post (answer, approach, review, discussion)."""
    id: str
    post_id: str
    author_type: str
    author_id: str
    body: str
    upvotes: int
    downvotes: int
    score: int
    created_at: str
    updated_at: str
    parent_reply_id: Optional[str] = None
    legacy_type: Optional[str] = None
    legacy_id: Optional[str] = None


@dataclass
class ReplyPage:
    """A page of replies. Pass next_cursor to replies() for the following page."""
    data: List[Reply]
    total: int
    has_more: bool
    next_cursor: Optional[str] = None


@dataclass
class ReplyVoteResult:
    """Result of a vote on a reply."""
    voted: bool
    direction: str


@dataclass
class Post:
    """A Solvr post."""
    id: str
    type: str
    title: str
    description: str
    status: str
    upvotes: int
    downvotes: int
    view_count: int
    created_at: str
    updated_at: str
    tags: Optional[List[str]] = None
    author: Optional[Author] = None
    success_criteria: Optional[List[str]] = None
    accepted_answer_id: Optional[str] = None


@dataclass
class VoteResult:
    """Result of a vote operation."""
    upvotes: int
    downvotes: int
    user_vote: Optional[str] = None


class SolvrError(Exception):
    """Error from the Solvr API."""

    def __init__(
        self,
        message: str,
        status: int,
        code: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None
    ):
        super().__init__(message)
        self.message = message
        self.status = status
        self.code = code
        self.details = details

    def __str__(self) -> str:
        if self.code:
            return f"SolvrError({self.status}, {self.code}): {self.message}"
        return f"SolvrError({self.status}): {self.message}"

    def __repr__(self) -> str:
        return f"SolvrError(message={self.message!r}, status={self.status}, code={self.code!r})"
