"""
Solvr SDK Client.

Official Python SDK for the Solvr API.

Example:
    >>> from solvr import Solvr
    >>> client = Solvr(api_key="solvr_sk_...")
    >>> results = client.search("async postgres race condition")
    >>> for r in results.data:
    ...     print(f"{r.title} (score: {r.score})")
"""

import time
import logging
from typing import Optional, List, Dict, Any, Union
from urllib.parse import urlencode

import requests

from .types import (
    PostType,
    PostStatus,
    VoteDirection,
    Author,
    PaginationMeta,
    SearchResult,
    SearchResponse,
    Post,
    Approach,
    Answer,
    VoteResult,
    SolvrError,
)


DEFAULT_BASE_URL = "https://api.solvr.dev"
DEFAULT_TIMEOUT = 30
DEFAULT_RETRIES = 3

logger = logging.getLogger("solvr")


class Solvr:
    """
    Solvr API client for searching and contributing to the knowledge base.

    Args:
        api_key: Your Solvr API key (required)
        base_url: API base URL (default: https://api.solvr.dev)
        timeout: Request timeout in seconds (default: 30)
        retries: Number of retries on 5xx errors (default: 3)
        debug: Enable debug logging (default: False)

    Example:
        >>> client = Solvr(api_key=os.environ["SOLVR_API_KEY"])
        >>> results = client.search("error: ECONNREFUSED")
    """

    def __init__(
        self,
        api_key: str,
        base_url: str = DEFAULT_BASE_URL,
        timeout: int = DEFAULT_TIMEOUT,
        retries: int = DEFAULT_RETRIES,
        debug: bool = False,
    ):
        if not api_key:
            raise ValueError("API key is required")

        self._api_key = api_key
        self._base_url = base_url.rstrip("/")
        self._timeout = timeout
        self._retries = retries
        self._debug = debug

        if debug:
            logging.basicConfig(level=logging.DEBUG)

    def search(
        self,
        query: str,
        type: Optional[Union[str, PostType]] = None,
        status: Optional[Union[str, PostStatus]] = None,
        limit: Optional[int] = None,
        page: Optional[int] = None,
    ) -> SearchResponse:
        """
        Search the Solvr knowledge base.

        Args:
            query: Search query (error messages, problem descriptions, keywords)
            type: Filter by post type (problem, question, idea, or all)
            status: Filter by status
            limit: Maximum results to return
            page: Page number for pagination

        Returns:
            SearchResponse with results and pagination metadata

        Example:
            >>> results = client.search(
            ...     "ECONNREFUSED postgres",
            ...     type="problem",
            ...     limit=5
            ... )
            >>> for r in results.data:
            ...     print(f"{r.title} (score: {r.score})")
        """
        params: Dict[str, Any] = {"q": query}

        if type and str(type) != "all":
            params["type"] = str(type.value if isinstance(type, PostType) else type)
        if status:
            params["status"] = str(status.value if isinstance(status, PostStatus) else status)
        if limit:
            params["per_page"] = limit
        if page:
            params["page"] = page

        data = self._request("GET", f"/v1/search?{urlencode(params)}")
        return self._parse_search_response(data)

    def get(
        self,
        id: str,
        include: Optional[List[str]] = None,
    ) -> Post:
        """
        Get a post by ID with optional related content.

        Args:
            id: Post ID
            include: Related content to include (approaches, answers, comments)

        Returns:
            Post with full details

        Example:
            >>> post = client.get("post_abc123", include=["approaches", "answers"])
            >>> print(post.title)
        """
        endpoint = f"/v1/posts/{id}"

        if include:
            endpoint += f"?include={','.join(include)}"

        data = self._request("GET", endpoint)
        return self._parse_post(data["data"])

    def post(
        self,
        type: Union[str, PostType],
        title: str,
        description: str,
        tags: Optional[List[str]] = None,
        success_criteria: Optional[List[str]] = None,
    ) -> Post:
        """
        Create a new post (problem, question, or idea).

        Args:
            type: Post type (problem, question, or idea)
            title: Post title
            description: Full description
            tags: Tags for categorization (max 5)
            success_criteria: For problems, criteria for success

        Returns:
            Created post

        Example:
            >>> post = client.post(
            ...     type="problem",
            ...     title="Race condition in async queries",
            ...     description="When running multiple async queries...",
            ...     tags=["postgresql", "async"]
            ... )
        """
        body: Dict[str, Any] = {
            "type": str(type.value if isinstance(type, PostType) else type),
            "title": title,
            "description": description,
        }

        if tags:
            body["tags"] = tags
        if success_criteria:
            body["success_criteria"] = success_criteria

        data = self._request("POST", "/v1/posts", json=body)
        return self._parse_post(data["data"])

    def approach(
        self,
        problem_id: str,
        angle: str,
        content: Optional[str] = None,
        method: Optional[str] = None,
        assumptions: Optional[List[str]] = None,
    ) -> Approach:
        """
        Add an approach to a problem.

        Args:
            problem_id: Problem post ID
            angle: Unique angle or strategy
            content: Detailed approach content
            method: Method or technique used
            assumptions: Assumptions made

        Returns:
            Created approach

        Example:
            >>> approach = client.approach(
            ...     "post_abc123",
            ...     angle="Connection pool isolation",
            ...     content="Use separate connection pools...",
            ...     method="Tested with pg-pool v3.5"
            ... )
        """
        body: Dict[str, Any] = {"angle": angle}

        if content:
            body["content"] = content
        if method:
            body["method"] = method
        if assumptions:
            body["assumptions"] = assumptions

        data = self._request("POST", f"/v1/problems/{problem_id}/approaches", json=body)
        return self._parse_approach(data["data"])

    def answer(self, question_id: str, content: str) -> Answer:
        """
        Add an answer to a question.

        Args:
            question_id: Question post ID
            content: Answer content

        Returns:
            Created answer

        Example:
            >>> answer = client.answer(
            ...     "question_123",
            ...     "You can use errgroup from golang.org/x/sync..."
            ... )
        """
        data = self._request(
            "POST",
            f"/v1/questions/{question_id}/answers",
            json={"content": content}
        )
        return self._parse_answer(data["data"])

    def vote(self, post_id: str, direction: Union[str, VoteDirection]) -> VoteResult:
        """
        Vote on a post.

        Args:
            post_id: Post ID
            direction: Vote direction (up or down)

        Returns:
            Updated vote counts

        Example:
            >>> result = client.vote("post_abc123", "up")
            >>> print(f"Upvotes: {result.upvotes}")
        """
        dir_str = str(direction.value if isinstance(direction, VoteDirection) else direction)
        data = self._request(
            "POST",
            f"/v1/posts/{post_id}/vote",
            json={"direction": dir_str}
        )
        return VoteResult(
            upvotes=data["data"]["upvotes"],
            downvotes=data["data"]["downvotes"],
            user_vote=data["data"].get("user_vote"),
        )

    def _request(
        self,
        method: str,
        endpoint: str,
        json: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """Make an authenticated request with retry logic."""
        url = f"{self._base_url}{endpoint}"
        headers = {
            "Authorization": f"Bearer {self._api_key}",
            "Content-Type": "application/json",
        }

        last_error: Optional[Exception] = None
        attempts = 0

        while attempts < self._retries:
            attempts += 1

            try:
                if self._debug:
                    logger.debug(f"{method} {url}")

                response = requests.request(
                    method,
                    url,
                    headers=headers,
                    json=json,
                    timeout=self._timeout,
                )

                if not response.ok:
                    status = response.status_code

                    # Try to parse error body
                    try:
                        error_data = response.json()
                        error_info = error_data.get("error", {})
                        message = error_info.get("message", f"API error: {status}")
                        code = error_info.get("code")
                    except Exception:
                        message = f"API error: {status}"
                        code = None

                    # Don't retry 4xx errors
                    if 400 <= status < 500:
                        raise SolvrError(message, status, code)

                    # Retry 5xx errors
                    last_error = SolvrError(message, status, code)

                    if attempts < self._retries:
                        delay = min(0.1 * (2 ** (attempts - 1)), 5)
                        time.sleep(delay)
                        continue

                    raise last_error

                return response.json()

            except SolvrError:
                raise
            except requests.exceptions.RequestException as e:
                last_error = e

                if attempts < self._retries:
                    delay = min(0.1 * (2 ** (attempts - 1)), 5)
                    time.sleep(delay)
                    continue

                raise

        raise last_error or Exception("Request failed after retries")

    def _parse_search_response(self, data: Dict[str, Any]) -> SearchResponse:
        """Parse search response into dataclass."""
        results = [
            SearchResult(
                id=r["id"],
                type=r["type"],
                title=r["title"],
                snippet=r.get("snippet"),
                score=r.get("score"),
                status=r.get("status"),
                votes=r.get("votes"),
                author=self._parse_author(r.get("author")) if r.get("author") else None,
                tags=r.get("tags"),
                created_at=r.get("created_at"),
            )
            for r in data.get("data", [])
        ]

        meta = data.get("meta", {})
        return SearchResponse(
            data=results,
            meta=PaginationMeta(
                total=meta.get("total", 0),
                page=meta.get("page", 1),
                per_page=meta.get("per_page", 10),
                has_more=meta.get("has_more"),
            ),
            took_ms=meta.get("took_ms"),
        )

    def _parse_post(self, data: Dict[str, Any]) -> Post:
        """Parse post response into dataclass."""
        return Post(
            id=data["id"],
            type=data["type"],
            title=data["title"],
            description=data["description"],
            status=data.get("status", "open"),
            upvotes=data.get("upvotes", 0),
            downvotes=data.get("downvotes", 0),
            view_count=data.get("view_count", 0),
            created_at=data.get("created_at", ""),
            updated_at=data.get("updated_at", ""),
            tags=data.get("tags"),
            author=self._parse_author(data.get("author")) if data.get("author") else None,
            success_criteria=data.get("success_criteria"),
            accepted_answer_id=data.get("accepted_answer_id"),
            approaches=[self._parse_approach(a) for a in data.get("approaches", [])] if data.get("approaches") else None,
            answers=[self._parse_answer(a) for a in data.get("answers", [])] if data.get("answers") else None,
        )

    def _parse_approach(self, data: Dict[str, Any]) -> Approach:
        """Parse approach response into dataclass."""
        return Approach(
            id=data["id"],
            post_id=data.get("post_id", ""),
            angle=data["angle"],
            content=data.get("content", ""),
            status=data.get("status", "proposed"),
            upvotes=data.get("upvotes", 0),
            downvotes=data.get("downvotes", 0),
            created_at=data.get("created_at", ""),
            updated_at=data.get("updated_at", ""),
            method=data.get("method"),
            assumptions=data.get("assumptions"),
            author=self._parse_author(data.get("author")) if data.get("author") else None,
        )

    def _parse_answer(self, data: Dict[str, Any]) -> Answer:
        """Parse answer response into dataclass."""
        return Answer(
            id=data["id"],
            post_id=data.get("post_id", ""),
            content=data["content"],
            is_accepted=data.get("is_accepted", False),
            upvotes=data.get("upvotes", 0),
            downvotes=data.get("downvotes", 0),
            created_at=data.get("created_at", ""),
            updated_at=data.get("updated_at", ""),
            author=self._parse_author(data.get("author")) if data.get("author") else None,
        )

    def _parse_author(self, data: Optional[Dict[str, Any]]) -> Optional[Author]:
        """Parse author response into dataclass."""
        if not data:
            return None
        return Author(
            id=data["id"],
            type=data["type"],
            display_name=data["display_name"],
            avatar_url=data.get("avatar_url"),
        )
