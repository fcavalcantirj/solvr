"""Tests for Solvr client."""

import pytest
import responses
from solvr import Solvr, SolvrError, PostType, VoteDirection


API_KEY = "solvr_sk_test_key"
BASE_URL = "https://api.solvr.dev"


class TestSolvrConstructor:
    """Tests for Solvr constructor."""

    def test_create_with_api_key(self):
        """Should create instance with API key."""
        client = Solvr(api_key=API_KEY)
        assert client is not None

    def test_raise_without_api_key(self):
        """Should raise if API key is missing."""
        with pytest.raises(ValueError, match="API key is required"):
            Solvr(api_key="")

    def test_custom_base_url(self):
        """Should allow custom base URL."""
        client = Solvr(api_key=API_KEY, base_url="https://custom.api.com")
        assert client._base_url == "https://custom.api.com"

    def test_strip_trailing_slash(self):
        """Should strip trailing slash from base URL."""
        client = Solvr(api_key=API_KEY, base_url="https://custom.api.com/")
        assert client._base_url == "https://custom.api.com"


class TestSearch:
    """Tests for search method."""

    @responses.activate
    def test_search_basic(self):
        """Should search with query only."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/search?q=test+query",
            json={
                "data": [
                    {"id": "post_1", "type": "problem", "title": "Test Problem", "score": 0.95}
                ],
                "meta": {"total": 1, "page": 1, "per_page": 10},
            },
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        result = client.search("test query")

        assert len(result.data) == 1
        assert result.data[0].title == "Test Problem"
        assert result.data[0].score == 0.95
        assert result.meta.total == 1

    @responses.activate
    def test_search_with_options(self):
        """Should search with options."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/search?q=test&type=problem&per_page=5&page=2",
            json={"data": [], "meta": {"total": 0, "page": 2, "per_page": 5}},
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        result = client.search("test", type="problem", limit=5, page=2)

        assert result.data == []
        assert result.meta.page == 2

    @responses.activate
    def test_search_with_enum(self):
        """Should accept PostType enum."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/search?q=test&type=question",
            json={"data": [], "meta": {"total": 0, "page": 1, "per_page": 10}},
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        client.search("test", type=PostType.QUESTION)

        assert len(responses.calls) == 1


class TestGet:
    """Tests for get method."""

    @responses.activate
    def test_get_post(self):
        """Should get post by ID."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/posts/post_123",
            json={
                "data": {
                    "id": "post_123",
                    "type": "problem",
                    "title": "Test",
                    "description": "Test description",
                    "status": "open",
                    "upvotes": 10,
                    "downvotes": 0,
                    "view_count": 100,
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z",
                }
            },
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        post = client.get("post_123")

        assert post.id == "post_123"
        assert post.title == "Test"
        assert post.upvotes == 10

    @responses.activate
    def test_get_with_includes(self):
        """Should get post with includes."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/posts/post_123?include=approaches,answers",
            json={
                "data": {
                    "id": "post_123",
                    "type": "problem",
                    "title": "Test",
                    "description": "Test",
                    "status": "open",
                    "upvotes": 0,
                    "downvotes": 0,
                    "view_count": 0,
                    "created_at": "",
                    "updated_at": "",
                }
            },
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        client.get("post_123", include=["approaches", "answers"])

        assert "include=approaches,answers" in responses.calls[0].request.url


class TestPost:
    """Tests for post method."""

    @responses.activate
    def test_create_post(self):
        """Should create a new post."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/posts",
            json={
                "data": {
                    "id": "post_new",
                    "type": "problem",
                    "title": "New Problem",
                    "description": "Problem description",
                    "status": "open",
                    "tags": ["typescript", "api"],
                    "upvotes": 0,
                    "downvotes": 0,
                    "view_count": 0,
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z",
                }
            },
            status=201,
        )

        client = Solvr(api_key=API_KEY)
        post = client.post(
            type="problem",
            title="New Problem",
            description="Problem description",
            tags=["typescript", "api"],
        )

        assert post.id == "post_new"
        assert post.tags == ["typescript", "api"]


class TestApproach:
    """Tests for approach method."""

    @responses.activate
    def test_add_approach(self):
        """Should add approach to a problem."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/problems/post_123/approaches",
            json={
                "data": {
                    "id": "approach_1",
                    "post_id": "post_123",
                    "angle": "Test angle",
                    "content": "Test content",
                    "status": "proposed",
                    "upvotes": 0,
                    "downvotes": 0,
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z",
                }
            },
            status=201,
        )

        client = Solvr(api_key=API_KEY)
        approach = client.approach(
            "post_123",
            angle="Test angle",
            content="Test content",
        )

        assert approach.id == "approach_1"
        assert approach.angle == "Test angle"


class TestAnswer:
    """Tests for answer method."""

    @responses.activate
    def test_add_answer(self):
        """Should add answer to a question."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/questions/question_123/answers",
            json={
                "data": {
                    "id": "answer_1",
                    "post_id": "question_123",
                    "content": "Test answer",
                    "is_accepted": False,
                    "upvotes": 0,
                    "downvotes": 0,
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z",
                }
            },
            status=201,
        )

        client = Solvr(api_key=API_KEY)
        answer = client.answer("question_123", "Test answer")

        assert answer.id == "answer_1"
        assert answer.content == "Test answer"


class TestVote:
    """Tests for vote method."""

    @responses.activate
    def test_upvote(self):
        """Should upvote a post."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/posts/post_123/vote",
            json={"data": {"upvotes": 11, "downvotes": 0, "user_vote": "up"}},
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        result = client.vote("post_123", "up")

        assert result.upvotes == 11
        assert result.user_vote == "up"

    @responses.activate
    def test_downvote_with_enum(self):
        """Should accept VoteDirection enum."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/posts/post_123/vote",
            json={"data": {"upvotes": 10, "downvotes": 1, "user_vote": "down"}},
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        result = client.vote("post_123", VoteDirection.DOWN)

        assert result.downvotes == 1


class TestErrorHandling:
    """Tests for error handling."""

    @responses.activate
    def test_api_error(self):
        """Should raise SolvrError on API error."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/posts/invalid_id",
            json={"error": {"message": "Not found", "code": "NOT_FOUND"}},
            status=404,
        )

        client = Solvr(api_key=API_KEY, retries=1)

        with pytest.raises(SolvrError) as exc_info:
            client.get("invalid_id")

        assert exc_info.value.status == 404
        assert exc_info.value.code == "NOT_FOUND"

    @responses.activate
    def test_non_json_error(self):
        """Should handle non-JSON error responses."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/posts/post_123",
            body="Internal Server Error",
            status=500,
        )

        client = Solvr(api_key=API_KEY, retries=1)

        with pytest.raises(SolvrError) as exc_info:
            client.get("post_123")

        assert exc_info.value.status == 500


class TestRetryLogic:
    """Tests for retry logic."""

    @responses.activate
    def test_retry_on_5xx(self):
        """Should retry on 5xx errors."""
        # First two fail, third succeeds
        responses.add(responses.GET, f"{BASE_URL}/v1/search?q=test", status=503)
        responses.add(responses.GET, f"{BASE_URL}/v1/search?q=test", status=503)
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/search?q=test",
            json={"data": [], "meta": {"total": 0, "page": 1, "per_page": 10}},
            status=200,
        )

        client = Solvr(api_key=API_KEY, retries=3)
        result = client.search("test")

        assert len(responses.calls) == 3
        assert result.data == []

    @responses.activate
    def test_no_retry_on_4xx(self):
        """Should not retry on 4xx errors."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/search?q=test",
            json={"error": {"message": "Unauthorized"}},
            status=401,
        )

        client = Solvr(api_key=API_KEY, retries=3)

        with pytest.raises(SolvrError):
            client.search("test")

        assert len(responses.calls) == 1

    @responses.activate
    def test_fail_after_max_retries(self):
        """Should fail after max retries."""
        responses.add(responses.GET, f"{BASE_URL}/v1/search?q=test", status=503)
        responses.add(responses.GET, f"{BASE_URL}/v1/search?q=test", status=503)

        client = Solvr(api_key=API_KEY, retries=2)

        with pytest.raises(SolvrError):
            client.search("test")

        assert len(responses.calls) == 2
