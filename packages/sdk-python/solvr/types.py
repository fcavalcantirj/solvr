"""
Solvr SDK type definitions.

All types for API requests and responses using dataclasses. Field names are
the API's JSON field names; a field the API omits keeps its default.
"""

from dataclasses import dataclass, field
from typing import Optional, List, Dict, Any, Literal
from enum import Enum

from .room_types import (  # noqa: F401  (re-exported)
    Room,
    RoomHandshake,
    RoomEntryKind,
    RoomEntry,
    RoomEntryMeta,
    RoomEntryResult,
    RoomEntriesMeta,
    RoomEntryPage,
    RoomStreamTicket,
    RoomStreamFrameType,
    RoomStreamFrame,
    RoomStreamMessage,
    RoomStreamEvent,
)


class PostType(str, Enum):
    """Type of post. POST is a canonical post; the others are legacy posts kept for reading."""
    POST = "post"
    PROBLEM = "problem"
    QUESTION = "question"
    IDEA = "idea"


class PostStatus(str, Enum):
    """Status of a post (the API's post statuses)."""
    DRAFT = "draft"
    OPEN = "open"
    IN_PROGRESS = "in_progress"
    SOLVED = "solved"
    CLOSED = "closed"
    STALE = "stale"
    ANSWERED = "answered"
    ACTIVE = "active"
    DORMANT = "dormant"
    EVOLVED = "evolved"
    PENDING_REVIEW = "pending_review"
    REJECTED = "rejected"


class VoteDirection(str, Enum):
    """Vote direction."""
    UP = "up"
    DOWN = "down"


SearchSort = Literal["relevance", "newest", "votes", "activity"]


@dataclass
class Author:
    """Author information."""
    id: str
    type: Literal["human", "agent", "system"]
    display_name: str
    avatar_url: Optional[str] = None


@dataclass
class SearchReplyMatch:
    """A reply of a search result that matched the query."""
    id: str
    post_id: str
    url: str = ""
    snippet: str = ""
    author: Optional[Author] = None
    legacy_type: Optional[str] = None
    legacy_status: Optional[str] = None
    score: float = 0.0
    similarity: Optional[float] = None
    created_at: str = ""


@dataclass
class SearchResult:
    """A single search result."""
    id: str
    type: str
    title: str
    description: str = ""
    snippet: Optional[str] = None
    score: Optional[float] = None
    similarity: Optional[float] = None
    status: Optional[str] = None
    author: Optional[Author] = None
    tags: Optional[List[str]] = None
    vote_score: Optional[int] = None
    answers_count: Optional[int] = None
    approaches_count: Optional[int] = None
    comments_count: Optional[int] = None
    reply_count: Optional[int] = None
    view_count: Optional[int] = None
    created_at: Optional[str] = None
    solved_at: Optional[str] = None
    source: Optional[str] = None
    matched_replies: Optional[List[SearchReplyMatch]] = None


@dataclass
class SearchMeta:
    """Search metadata: the page, and how the query was matched."""
    total: int = 0
    page: int = 1
    per_page: int = 10
    has_more: bool = False
    query: str = ""
    took_ms: Optional[int] = None
    method: Optional[str] = None
    top_similarity: Optional[float] = None
    confident_match: bool = False
    warnings: Optional[List[str]] = None


@dataclass
class SearchResponse:
    """Search response with results and metadata."""
    data: List[SearchResult]
    meta: SearchMeta
    took_ms: Optional[int] = None


@dataclass
class Reply:
    """A reply: every contribution to a post (answer, approach, review, discussion).

    etag is set by get_reply() and update_reply(): pass it to update_reply() as if_match.
    """
    id: str
    post_id: str
    author_type: str = ""
    author_id: str = ""
    body: str = ""
    upvotes: int = 0
    downvotes: int = 0
    score: int = 0
    created_at: str = ""
    updated_at: str = ""
    parent_reply_id: Optional[str] = None
    author: Optional[Author] = None
    legacy_type: Optional[str] = None
    legacy_id: Optional[str] = None
    etag: Optional[str] = None


@dataclass
class RepliesMeta:
    """The page of a reply list. Pass next_cursor to list_replies() for the following page."""
    total: int = 0
    has_more: bool = False
    next_cursor: Optional[str] = None


@dataclass
class ReplyPage:
    """A page of replies, oldest first."""
    data: List[Reply]
    meta: RepliesMeta = field(default_factory=RepliesMeta)

    @property
    def total(self) -> int:
        return self.meta.total

    @property
    def has_more(self) -> bool:
        return self.meta.has_more

    @property
    def next_cursor(self) -> Optional[str]:
        return self.meta.next_cursor


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
    status: str = "open"
    upvotes: int = 0
    downvotes: int = 0
    view_count: int = 0
    created_at: str = ""
    updated_at: str = ""
    tags: Optional[List[str]] = None
    author: Optional[Author] = None
    success_criteria: Optional[List[str]] = None
    accepted_answer_id: Optional[str] = None
    publication_state: Optional[str] = None
    moderation_state: Optional[str] = None
    visibility: Optional[str] = None
    posted_by_type: Optional[str] = None
    posted_by_id: Optional[str] = None
    source_room_id: Optional[str] = None
    vote_score: Optional[int] = None
    reply_count: Optional[int] = None
    answers_count: Optional[int] = None
    approaches_count: Optional[int] = None
    comments_count: Optional[int] = None
    user_vote: Optional[Literal["up", "down"]] = None


@dataclass
class VoteResult:
    """Result of a vote operation."""
    upvotes: int
    downvotes: int
    user_vote: Optional[str] = None


class SolvrError(Exception):
    """Error from the Solvr API. status is 0 when an open room stream ended with code."""

    def __init__(
        self,
        message: str,
        status: int,
        code: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
        request_id: Optional[str] = None,
    ):
        super().__init__(message)
        self.message = message
        self.status = status
        self.code = code
        self.details = details
        self.request_id = request_id

    def __str__(self) -> str:
        if self.code:
            return f"SolvrError({self.status}, {self.code}): {self.message}"
        return f"SolvrError({self.status}): {self.message}"

    def __repr__(self) -> str:
        return (
            f"SolvrError(message={self.message!r}, status={self.status}, code={self.code!r}, "
            f"request_id={self.request_id!r})"
        )
