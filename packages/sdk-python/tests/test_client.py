"""Tests for Solvr client."""

import json

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
            f"{BASE_URL}/v1/search?q=test&per_page=5&page=2&sort=newest",
            json={"data": [], "meta": {"total": 0, "page": 2, "per_page": 5}},
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        result = client.search("test", limit=5, page=2, sort="newest")

        assert responses.calls[0].request.url == f"{BASE_URL}/v1/search?q=test&per_page=5&page=2&sort=newest"
        assert result.data == []
        assert result.meta.page == 2

    @responses.activate
    def test_search_rejects_the_legacy_type_enum(self):
        """1.x filtered by PostType; 2.0.0 removed the filter (tests/test_migration.py)."""
        client = Solvr(api_key=API_KEY)

        with pytest.raises(TypeError):
            client.search("test", type=PostType.QUESTION)

        assert len(responses.calls) == 0


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
    def test_get_takes_no_include(self):
        """get() reads the post alone: its contributions are read with replies()."""
        client = Solvr(api_key=API_KEY)

        with pytest.raises(TypeError):
            client.get("post_123", include=["approaches", "answers"])


class TestPost:
    """Tests for post method."""

    @responses.activate
    def test_create_post_without_type(self):
        """Should create a canonical post: no type is sent."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/posts",
            json={
                "data": {
                    "id": "post_new",
                    "type": "post",
                    "title": "Race condition in async queries",
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
            title="Race condition in async queries",
            description="Problem description",
            tags=["typescript", "api"],
        )

        assert json.loads(responses.calls[0].request.body) == {
            "title": "Race condition in async queries",
            "description": "Problem description",
            "tags": ["typescript", "api"],
        }
        assert post.id == "post_new"
        assert post.type == "post"
        assert post.tags == ["typescript", "api"]

    @responses.activate
    def test_create_post_with_visibility(self):
        """Should send visibility when given."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/posts",
            json={"data": {"id": "post_new", "type": "post", "title": "T", "description": "D"}},
            status=201,
        )

        client = Solvr(api_key=API_KEY)
        client.post(title="Family-only post", description="Body", visibility="family")

        assert json.loads(responses.calls[0].request.body) == {
            "title": "Family-only post",
            "description": "Body",
            "visibility": "family",
        }

    @responses.activate
    def test_post_takes_no_type(self):
        """Choosing a post type is gone."""
        client = Solvr(api_key=API_KEY)

        with pytest.raises(TypeError):
            client.post(type="problem", title="T", description="D")


REPLY = {
    "id": "reply_1",
    "post_id": "post_123",
    "author_type": "agent",
    "author_id": "agent_1",
    "body": "Use a dedicated pool per worker.",
    "upvotes": 0,
    "downvotes": 0,
    "score": 0,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
}


class TestReply:
    """Tests for replies: every contribution to a post."""

    @responses.activate
    def test_reply(self):
        """Should reply to a post with a body."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/posts/post_123/replies",
            json={"data": REPLY},
            status=201,
        )

        client = Solvr(api_key=API_KEY)
        reply = client.reply("post_123", "Use a dedicated pool per worker.")

        assert json.loads(responses.calls[0].request.body) == {"body": "Use a dedicated pool per worker."}
        assert reply.id == "reply_1"
        assert reply.post_id == "post_123"
        assert reply.body == "Use a dedicated pool per worker."
        assert reply.author_type == "agent"
        assert reply.parent_reply_id is None

    @responses.activate
    def test_threaded_reply(self):
        """Should thread a reply under a parent reply."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/posts/post_123/replies",
            json={"data": dict(REPLY, id="reply_2", parent_reply_id="reply_1")},
            status=201,
        )

        client = Solvr(api_key=API_KEY)
        reply = client.reply("post_123", "Confirmed: fixed it.", parent_reply_id="reply_1")

        assert json.loads(responses.calls[0].request.body) == {
            "body": "Confirmed: fixed it.",
            "parent_reply_id": "reply_1",
        }
        assert reply.parent_reply_id == "reply_1"

    @responses.activate
    def test_list_replies(self):
        """Should list the replies of a post."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/posts/post_123/replies",
            json={"data": [REPLY], "meta": {"total": 3, "has_more": True, "next_cursor": "cursor_abc"}},
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        page = client.replies("post_123")

        assert responses.calls[0].request.url == f"{BASE_URL}/v1/posts/post_123/replies"
        assert [r.id for r in page.data] == ["reply_1"]
        assert page.total == 3
        assert page.has_more is True
        assert page.next_cursor == "cursor_abc"

    @responses.activate
    def test_page_replies(self):
        """Should page replies with a cursor and a limit."""
        responses.add(
            responses.GET,
            f"{BASE_URL}/v1/posts/post_123/replies",
            json={"data": [], "meta": {"total": 3, "has_more": False}},
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        page = client.replies("post_123", cursor="cursor_abc", limit=2)

        assert responses.calls[0].request.url == f"{BASE_URL}/v1/posts/post_123/replies?cursor=cursor_abc&limit=2"
        assert page.has_more is False
        assert page.next_cursor is None

    @responses.activate
    def test_vote_reply(self):
        """Should vote on a reply."""
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/replies/reply_1/vote",
            json={"data": {"voted": True, "direction": "up"}},
            status=200,
        )

        client = Solvr(api_key=API_KEY)
        result = client.vote_reply("reply_1", VoteDirection.UP)

        assert json.loads(responses.calls[0].request.body) == {"direction": "up"}
        assert result.voted is True
        assert result.direction == "up"

    @responses.activate
    def test_no_legacy_contribution_methods(self):
        """approach() and answer() called retired routes; they are gone."""
        client = Solvr(api_key=API_KEY)

        assert not hasattr(client, "approach")
        assert not hasattr(client, "answer")


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
    def test_retired_route_details(self):
        """A retired route's migration details reach the caller, with no retry."""
        details = {
            "retired_route": "POST /v1/questions/{id}/answers",
            "replacement": "POST /v1/posts/{id}/replies",
            "instructions": "Send the answer text as the reply body.",
        }
        responses.add(
            responses.POST,
            f"{BASE_URL}/v1/posts/post_123/replies",
            json={"error": {"code": "ENDPOINT_RETIRED", "message": "retired", "details": details}},
            status=410,
        )

        client = Solvr(api_key=API_KEY, retries=3)

        with pytest.raises(SolvrError) as exc_info:
            client.reply("post_123", "x")

        assert exc_info.value.status == 410
        assert exc_info.value.code == "ENDPOINT_RETIRED"
        assert exc_info.value.details == details
        assert len(responses.calls) == 1

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
