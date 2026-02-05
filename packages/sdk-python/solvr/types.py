"""
Solvr SDK type definitions.

All types for API requests and responses using dataclasses.
"""

from dataclasses import dataclass, field
from typing import Optional, List, Dict, Any, Literal
from enum import Enum


class PostType(str, Enum):
    """Type of post."""
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


class ApproachStatus(str, Enum):
    """Status of an approach."""
    PROPOSED = "proposed"
    IN_PROGRESS = "in_progress"
    VALIDATED = "validated"
    REJECTED = "rejected"


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
class Approach:
    """An approach to solving a problem."""
    id: str
    post_id: str
    angle: str
    content: str
    status: str
    upvotes: int
    downvotes: int
    created_at: str
    updated_at: str
    method: Optional[str] = None
    assumptions: Optional[List[str]] = None
    author: Optional[Author] = None


@dataclass
class Answer:
    """An answer to a question."""
    id: str
    post_id: str
    content: str
    is_accepted: bool
    upvotes: int
    downvotes: int
    created_at: str
    updated_at: str
    author: Optional[Author] = None


@dataclass
class Comment:
    """A comment on a post, approach, or answer."""
    id: str
    target_type: str
    target_id: str
    content: str
    created_at: str
    author: Optional[Author] = None


@dataclass
class Post:
    """A Solvr post (problem, question, or idea)."""
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
    approaches: Optional[List[Approach]] = None
    answers: Optional[List[Answer]] = None
    comments: Optional[List[Comment]] = None


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
