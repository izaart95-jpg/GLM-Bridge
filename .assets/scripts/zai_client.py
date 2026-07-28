import os
import sys
import time
import requests
import json
import argparse
import threading
import uuid
import urllib.parse
import re
from typing import AsyncGenerator
from fastapi import FastAPI, Request, Response
from fastapi.responses import StreamingResponse
from fastapi.middleware.cors import CORSMiddleware
import uvicorn
from uvicorn.config import LOGGING_CONFIG
import httpx
from openai import OpenAI
from rich.console import Console
from rich.panel import Panel
from rich.table import Table
from rich.theme import Theme
try:
    from bs4 import BeautifulSoup
except ImportError:
    BeautifulSoup = None

# ─────────────────────────────────────────────────────────────────────────────
# CONFIGURATION & THEME
# ─────────────────────────────────────────────────────────────────────────────

DEFAULT_BASE_URL = "http://localhost:3001/v1"
DEFAULT_API_KEY = "Waguri"
UPSTREAM_BASE_URL = DEFAULT_BASE_URL
UPSTREAM_API_KEY = DEFAULT_API_KEY

custom_theme = Theme({
    "thinking": "italic grey50",
    "answer": "green",
    "header": "bold cyan",
    "meta": "bold magenta",
    "info": "yellow",
    "error": "bold red",
    "success": "bold green",
    "tool": "bold magenta"
})
console = Console(theme=custom_theme)
VERBOSE = False


def log_verbose(message: str, payload: object | None = None):
    """Print verbose bridge diagnostics when requested."""
    if not VERBOSE:
        return

    console.print(f"[dim][verbose]{message}[/dim]")
    if payload is None:
        return

    try:
        rendered = json.dumps(payload, ensure_ascii=False, indent=2)
    except Exception:
        rendered = str(payload)

    console.print(Panel.fit(rendered, title="Verbose Payload", border_style="magenta"))


# Helper to generate unique transaction IDs
def generateID() -> str:
    return uuid.uuid4().hex

# ─────────────────────────────────────────────────────────────────────────────
# STANDARDIZED NATIVE OPENAI TOOLS SCHEMAS (FOR SHELL AGENT MODE)
# ─────────────────────────────────────────────────────────────────────────────

# ── Available Tools ────────────────────────────────────────────────────────────────
# Only web-based tools: `fetch` (direct URL fetch) and `websearch` (search + browse)
# WebSearch is implemented from scratch using Python packages - NOT via GLM's native webSearch flag
# (which causes 500 errors with the zai-proxy bridge)

TOOLS_SCHEMA = [
    {
        "type": "function",
        "function": {
            "name": "fetch",
            "description": "Fetches content from a URL via HTTP GET and returns the response body. Use for web research, content retrieval, and URL validation.",
            "parameters": {
                "type": "object",
                "properties": {
                    "url": {
                        "type": "string",
                        "description": "The URL to fetch content from (e.g., https://example.com)."
                    },
                    "max_length": {
                        "type": "integer",
                        "default": 15000,
                        "description": "Maximum number of bytes to return in the response."
                    }
                },
                "required": ["url"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "websearch",
            "description": "Performs a web search and returns the top results. Use when you need to find information online or browse specific topics. Returns a list of search results with titles, URLs, and snippets.",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "The search query (e.g., 'latest AI news 2024', 'Python async tutorial')."
                    },
                    "num_results": {
                        "type": "integer",
                        "default": 5,
                        "description": "Number of search results to return (max 10)."
                    },
                    "fetch_content": {
                        "type": "boolean",
                        "default": False,
                        "description": "Whether to fetch and include the full content of the top result."
                    }
                },
                "required": ["query"]
            }
        }
    }
]

def execute_local_tool(name: str, arguments: dict) -> str:
    """Safely executes the requested tool locally.

    Supports built-in file tools and mock MCP-style tools for testing.
    """
    try:
        # ── Built-in File Tools ─────────────────────────────────────
        if name == "read_file":
            path = arguments.get("path")
            if not path:
                return "Error: Missing required argument 'path'."
            clean_path = os.path.normpath(path)
            with open(clean_path, "r", encoding="utf-8") as f:
                return f.read()

        elif name == "write_file":
            path = arguments.get("path")
            content = arguments.get("content", "")
            if not path:
                return "Error: Missing required argument 'path'."
            clean_path = os.path.normpath(path)

            # Ensure parent directories exist
            dir_name = os.path.dirname(clean_path)
            if dir_name:
                os.makedirs(dir_name, exist_ok=True)

            with open(clean_path, "w", encoding="utf-8") as f:
                f.write(content)
            return f"Success: Successfully wrote {len(content)} characters to '{path}'."

        # ── Web Tools ────────────────────────────────────────────────────────────────
        elif name == "fetch":
            # Real HTTP fetch with gzip auto-decompression using requests
            url = arguments.get("url", "")
            max_length = arguments.get("max_length", 15000)
            if not url:
                return "Error: Missing required argument 'url'."
            try:
                import requests
                headers = {
                    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
                    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
                    'Accept-Language': 'en-US,en;q=0.5',
                    'Accept-Encoding': 'gzip, deflate',  # requests auto-decompresses
                    'Connection': 'keep-alive',
                }
                response = requests.get(url, headers=headers, timeout=30, stream=True)
                # Check content length before downloading
                content_length = response.headers.get('Content-Length')
                if content_length and int(content_length) > 5_000_000:
                    return f"[fetch ERROR] Content too large ({content_length} bytes)"

                content = response.text  # requests auto-decompresses gzip/deflate
                # Sanitize content to prevent Rich markup errors
                content = _sanitize_for_rich(content)

                if len(content) > max_length:
                    content = content[:max_length] + "\n... [truncated]"
                return f"[fetch] URL: {url}\nStatus: {response.status_code}\nHeaders: {dict(response.headers)}\n\nContent:\n{content}"
            except Exception as e:
                return f"[fetch ERROR] Failed to fetch {url}: {e}"

        elif name == "websearch":
            query = arguments.get("query", "")
            num_results = min(arguments.get("num_results", 5), 10)
            fetch_content = arguments.get("fetch_content", False)

            if not query:
                return "Error: Missing required argument 'query'."

            if BeautifulSoup is None:
                return "[websearch ERROR] BeautifulSoup not installed. Run: pip install beautifulsoup4"

            try:
                headers = {
                    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
                    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
                    'Connection': 'close',
                }

                results = []
                search_engines = [
                    (f"https://duckduckgo.com/html/?q={urllib.parse.quote(query)}", "ddg_html"),
                    (f"https://www.bing.com/search?q={urllib.parse.quote(query)}", "bing"),
                ]

                for search_url, engine_name in search_engines:
                    try:
                        resp = requests.get(search_url, headers=headers, timeout=5)
                        resp.raise_for_status()
                        soup = BeautifulSoup(resp.text, 'html.parser')

                        results = []
                        if engine_name == "ddg_html":
                            for idx, link in enumerate(soup.select('.result__a'), 1):
                                if idx > num_results:
                                    break
                                url = link.get('href')
                                title = link.get_text(strip=True)
                                if url and title:
                                    parent = link.find_parent(class_='result')
                                    snippet_elem = parent.find(class_='result__snippet') if parent else None
                                    snippet = snippet_elem.get_text(strip=True)[:200] if snippet_elem else ''
                                    results.append({"title": title, "url": url, "snippet": snippet, "index": len(results)+1})

                        elif engine_name == "ddg_lite":
                            for idx, row in enumerate(soup.select('table tr'), 1):
                                if idx > num_results:
                                    break
                                cells = row.find_all('td')
                                if len(cells) >= 2:
                                    link = cells[1].find('a')
                                    if link:
                                        url = link.get('href')
                                        title = link.get_text(strip=True)
                                        snippet = cells[2].get_text(strip=True)[:200] if len(cells) > 2 else ''
                                        results.append({"title": title, "url": url, "snippet": snippet, "index": len(results)+1})

                        elif engine_name == "brave":
                            for idx, link in enumerate(soup.select('.snippet-title'), 1):
                                if idx > num_results:
                                    break
                                url_elem = link.find_parent('a')
                                if url_elem:
                                    url = url_elem.get('href')
                                    title = link.get_text(strip=True)
                                    if url and title:
                                        results.append({"title": title, "url": url, "snippet": "", "index": len(results)+1})

                        elif engine_name == "bing":
                            for idx, link in enumerate(soup.select('.b_algo h2 a'), 1):
                                if idx > num_results:
                                    break
                                url = link.get('href')
                                title = link.get_text(strip=True)
                                if url and title:
                                    results.append({"title": title, "url": url, "snippet": "", "index": len(results)+1})

                        if results:
                            break

                    except Exception:
                        continue

                if not results:
                    return (f"[websearch] No results found for: {query}\n\n"
                            f"Try:\n1. Rephrasing your query\n"
                            f"2. Using the 'fetch' tool with a known URL\n"
                            f"3. Checking network connectivity")

                # Fetch content for top result
                if fetch_content and results and results[0].get('url'):
                    try:
                        content_resp = requests.get(results[0]['url'], headers=headers, timeout=5)
                        content_resp.raise_for_status()
                        content_text = _extract_main_text(content_resp.text, max_len=5000)
                        results[0]["content"] = content_text
                    except Exception as e:
                        results[0]["content_fetch_error"] = str(e)

                # Format output
                lines = [f"[websearch] Results for: {query}", ""]
                for r in results:
                    title = _sanitize_for_rich(r.get('title', ''))
                    url = r.get('url', '')
                    snippet = _sanitize_for_rich(r.get('snippet', ''))
                    lines.append(f"--- Result {r.get('index', 0)} ---")
                    lines.append(f"Title: {title}")
                    lines.append(f"URL: {url}")
                    if snippet:
                        lines.append(f"Snippet: {snippet}")
                    if "content" in r:
                        content = _sanitize_for_rich(r['content'])
                        lines.append(f"Content Preview: {content[:1000]}...")
                    lines.append("")

                return "\n".join(lines)

            except Exception as e:
                return f"[websearch ERROR] Failed to search '{query}': {e}"

        else:
            return f"Error: Unknown tool '{name}'."
    except Exception as e:
        return f"Error executing local tool '{name}': {e}"



def _sanitize_for_rich(text: str) -> str:
    """Sanitize text to prevent Rich markup parsing errors from binary/gzip content."""
    if not text:
        return ""
    import re
    # Remove null bytes and control characters except newlines/tabs
    text = text.replace('\x00', '')
    text = re.sub(r'[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]', '', text)
    # Escape Rich markup brackets that could cause parsing errors
    text = re.sub(r'\[/?[a-zA-Z][a-zA-Z0-9 ]*\]', '', text)
    # Limit consecutive newlines
    text = re.sub(r'\n{4,}', '\n\n\n', text)
    return text


def _extract_main_text(html_content: str, max_len: int = 5000) -> str:
    """Extract clean text from HTML, preferring main content areas."""
    if BeautifulSoup is None:
        return "[ERROR] BeautifulSoup not installed. Run: pip install beautifulsoup4"
    soup = BeautifulSoup(html_content, 'html.parser')
    for selector in ['main', 'article', '.content', '[role="main"]', '.post', '.entry', 'body']:
        elem = soup.select_one(selector)
        if elem:
            text = elem.get_text(' ', strip=True)
            text = _sanitize_for_rich(text)
            if len(text) > max_len:
                text = text[:max_len] + "... [truncated]"
            return text
    # Fallback: all text
    text = soup.get_text(' ', strip=True)
    text = _sanitize_for_rich(text)
    if len(text) > max_len:
        text = text[:max_len] + "... [truncated]"
    return text


# ─────────────────────────────────────────────────────────────────────────────
# INCREMENTAL STREAM PARSER (FOR TERMINAL PRESENTATION)
# ─────────────────────────────────────────────────────────────────────────────

class StreamFormatter:
    """
    Processes incoming text streams, parsing out <details type="reasoning">...</details>
    thinking blocks cleanly.
    """
    OPEN_PARTIALS = ["<", "<d", "<de", "<det", "<deta", "<detai", "<detail", "<details"]
    CLOSE_PARTIALS = ["<", "</", "</d", "</de", "</det", "</deta", "</detai", "</detail", "</details"]

    def __init__(self):
        self.buffer = ""
        self.in_thinking = False
        self.total_chars = 0

    def feed(self, text: str):
        self.buffer += text
        self.total_chars += len(text)
        
        while True:
            if not self.in_thinking:
                idx = self.buffer.find("<details")
                if idx != -1:
                    before = self.buffer[:idx]
                    if before:
                        console.print(before, end="", style="answer")
                        sys.stdout.flush()
                    
                    close_idx = self.buffer.find(">", idx)
                    if close_idx != -1:
                        self.buffer = self.buffer[close_idx + 1:]
                        self.in_thinking = True
                        console.print("\n\n[bold yellow]🤔 Thinking Process:[/bold yellow]\n", style="header")
                        sys.stdout.flush()
                        continue
                    else:
                        self.buffer = self.buffer[idx:]
                        break
                else:
                    matched_partial = False
                    for p in reversed(self.OPEN_PARTIALS):
                        if self.buffer.endswith(p):
                            to_print = self.buffer[:-len(p)]
                            if to_print:
                                console.print(to_print, end="", style="answer")
                                sys.stdout.flush()
                            self.buffer = p
                            matched_partial = True
                            break
                    if not matched_partial:
                        console.print(self.buffer, end="", style="answer")
                        sys.stdout.flush()
                        self.buffer = ""
                    break
            else:
                idx = self.buffer.find("</details>")
                if idx != -1:
                    thinking_text = self.buffer[:idx]
                    if thinking_text:
                        console.print(thinking_text, end="", style="thinking")
                        sys.stdout.flush()
                    
                    self.buffer = self.buffer[idx + len("</details>"):]
                    self.in_thinking = False
                    console.print("\n\n[bold green]💬 Response:[/bold green]\n", style="header")
                    sys.stdout.flush()
                    continue
                else:
                    matched_partial = False
                    for p in reversed(self.CLOSE_PARTIALS):
                        if self.buffer.endswith(p):
                            to_print = self.buffer[:-len(p)]
                            if to_print:
                                console.print(to_print, end="", style="thinking")
                                sys.stdout.flush()
                            self.buffer = p
                            matched_partial = True
                            break
                    if not matched_partial:
                        console.print(self.buffer, end="", style="thinking")
                        sys.stdout.flush()
                        self.buffer = ""
                    break

    def flush(self):
        if self.buffer:
            style = "thinking" if self.in_thinking else "answer"
            console.print(self.buffer, end="", style=style)
            sys.stdout.flush()
            self.buffer = ""

# ─────────────────────────────────────────────────────────────────────────────
# BACKEND UTILITIES & HEALTH CHECKS
# ─────────────────────────────────────────────────────────────────────────────

def run_health_check(url: str) -> bool:
    """Verifies connection health with the local Go Bridge."""
    status_url = f"{url}/status".replace("/v1/status", "/status")
    try:
        response = requests.get(status_url, timeout=3)
        if response.status_code == 200:
            data = response.json()
            console.print(f"[success]✓[/success] Go Bridge detected (User: {data.get('userName', 'Unknown')})")
            return True
    except requests.exceptions.RequestException:
        pass
    
    console.print(f"[error]✗ Error:[/error] Could not connect to Go Bridge on [cyan]{status_url}[/cyan].")
    console.print("  Please make sure your bridge is running. (e.g., compile & run [green]zai-bridge[/green])\n")
    return False


def get_available_models(client: OpenAI) -> list:
    """Hits /v1/models using credentials to fetch active endpoints."""
    try:
        models_data = client.models.list()
        return [model.id for model in models_data.data]
    except Exception as e:
        console.print(f"[error]✗ Error retrieving models:[/error] {e}")
        console.print("[info]Falling back to standard configuration models.[/info]")
        return ["glm-5.2", "GLM-5.1", "GLM-5-Turbo", "GLM-5v-Turbo", "glm-4.7"]


def select_model(models: list, default_model: str | None = None) -> str:
    """Interactive CLI menu to select target models."""
    if default_model and default_model in models:
        return default_model

    table = Table(title="Available Models", show_header=True, header_style="bold magenta")
    table.add_column("Index", justify="center", style="cyan")
    table.add_column("Model Name", style="white")

    for idx, model_id in enumerate(models, 1):
        table.add_row(str(idx), model_id)

    console.print(table)
    while True:
        try:
            choice = input("\nSelect model index to test [default: 1]: ").strip()
            if choice == "":
                return models[0]
            choice_idx = int(choice) - 1
            if 0 <= choice_idx < len(models):
                return models[choice_idx]
            console.print(f"[error]Invalid range.[/error] Enter a value between 1 and {len(models)}.")
        except ValueError:
            console.print("[error]Invalid integer input.[/error]")


def enable_history_persistence(model: str, url: str, api_key: str):
    """Hits /features to enable history persistence for the target model."""
    base_url = url.replace("/v1", "")
    features_url = f"{base_url}/features"
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json"
    }
    body = {
        "model": model,
        "persistHistory": True
    }
    try:
        response = requests.post(features_url, headers=headers, json=body, timeout=5)
        if response.status_code == 200:
            console.print(f"[success]✓[/success] Active Session History: [bold cyan]PERSIST_ON[/bold cyan] (Model: {model})")
    except Exception as e:
        console.print(f"[error]✗ Warning:[/error] Failed to auto-enable history persistence: {e}")


def clear_active_session(session_id: str, url: str, api_key: str):
    """Hits /admin/clients/<session_id>/clear to wipe the active thread's history."""
    base_url = url.replace("/v1", "")
    clear_url = f"{base_url}/admin/clients/{session_id}/clear"
    headers = {
        "Authorization": f"Bearer {api_key}"
    }
    try:
        response = requests.post(clear_url, headers=headers, timeout=5)
        if response.status_code == 200:
            console.print(f"[success]✓[/success] Cleared session history for thread: [bold cyan]{session_id}[/bold cyan]")
    except Exception as e:
        console.print(f"[error]✗ Error clearing session:[/error] {e}")


def clear_all_sessions(url: str, api_key: str):
    """Hits /admin/session/clear to wipe all global conversation histories on the bridge."""
    base_url = url.replace("/v1", "")
    clear_url = f"{base_url}/admin/session/clear"
    headers = {
        "Authorization": f"Bearer {api_key}"
    }
    try:
        response = requests.post(clear_url, headers=headers, timeout=5)
        if response.status_code == 200:
            console.print("[success]✓[/success] Cleared [bold red]ALL[/bold red] conversation threads on the bridge.")
    except Exception as e:
        console.print(f"[error]✗ Error clearing global sessions:[/error] {e}")

# ─────────────────────────────────────────────────────────────────────────────
# CORE STREAM EVALUATOR & AGENT RECURSION LOOP
# ─────────────────────────────────────────────────────────────────────────────

def evaluate_prompt(client: OpenAI, model: str, messages: list, think: bool, search: bool, session_id: str, fresh_session: bool, use_tools: bool):
    """
    Executes streamed completions, handles tool call execution locally,
    and automatically feeds the results back to the session history recursively.
    """
    start_time = time.time()
    ttft = None
    char_count = 0
    formatter = StreamFormatter()
    
    active_tool_calls = {}

    try:
        api_kwargs = {
            "model": model,
            "messages": messages,
            "stream": True,
            "extra_headers": {
                "X-Session-Id": session_id,
                "X-Fresh-Session": "true" if fresh_session else "false"
            },
            "extra_body": {
                "deepThink": think,
                "webSearch": search
            }
        }
        
        if use_tools:
            api_kwargs["tools"] = TOOLS_SCHEMA
            api_kwargs["tool_choice"] = "auto"

        stream = client.chat.completions.create(**api_kwargs)

        for chunk in stream:
            if chunk.choices and len(chunk.choices) > 0:
                delta = chunk.choices[0].delta
                
                if delta.content:
                    if ttft is None:
                        ttft = time.time() - start_time
                    char_count += len(delta.content)
                    formatter.feed(delta.content)
                
                if delta.tool_calls:
                    for tc in delta.tool_calls:
                        idx = tc.index
                        if idx not in active_tool_calls:
                            active_tool_calls[idx] = {"id": "", "name": "", "arguments": ""}
                        
                        if tc.id:
                            active_tool_calls[idx]["id"] = tc.id
                        if tc.function:
                            if tc.function.name:
                                active_tool_calls[idx]["name"] += tc.function.name
                                console.print(f"\n[bold magenta]🛠️  Invoking Tool [{active_tool_calls[idx]['name']}]:[/bold magenta] ", end="")
                                sys.stdout.flush()
                            if tc.function.arguments:
                                active_tool_calls[idx]["arguments"] += tc.function.arguments
                                console.print(".", end="")
                                sys.stdout.flush()

        formatter.flush()
        print()

    except Exception as e:
        console.print(f"\n[error]✗ Stream evaluation failed:[/error] {e}\n")
        return

    duration = time.time() - start_time

    if not active_tool_calls:
        est_tokens = char_count // 4
        tps = est_tokens / duration if duration > 0 else 0

        metrics_table = Table(show_header=True, header_style="bold magenta", box=None)
        metrics_table.add_column("Metric", style="cyan")
        metrics_table.add_column("Value", style="white")

        metrics_table.add_row("Time to First Token (TTFT)", f"{ttft:.3f}s" if ttft else "N/A")
        metrics_table.add_row("Total Generation Time", f"{duration:.3f}s")
        metrics_table.add_row("Characters Received", f"{char_count} chars")
        metrics_table.add_row("Estimated Tokens Generated", f"~{est_tokens} tokens")
        metrics_table.add_row("Throughput Speed", f"{tps:.2f} tokens/sec")

        console.print(Panel(
            metrics_table,
            title="📊 Evaluation Metrics",
            border_style="green",
            expand=False
        ))
        return

    tool_calls_payload = []
    for idx, tc in active_tool_calls.items():
        tool_calls_payload.append({
            "id": tc["id"],
            "type": "function",
            "function": {
                "name": tc["name"],
                "arguments": tc["arguments"]
            }
        })
    
    messages.append({
        "role": "assistant",
        "tool_calls": tool_calls_payload
    })

    for idx, tc in active_tool_calls.items():
        name = tc["name"]
        raw_args = tc["arguments"]
        
        console.print(f"\n[bold yellow]⚡ Local Executor Running Tool '{name}'...[/bold yellow]")
        try:
            parsed_args = json.loads(raw_args)
        except Exception:
            parsed_args = {}
            
        result = execute_local_tool(name, parsed_args)
        console.print(f"[success]✓ Result Output ({len(result)} chars):[/success]\n[dim white]{result}[/dim white]")
        
        messages.append({
            "role": "tool",
            "tool_call_id": tc["id"],
            "name": name,
            "content": result
        })

    evaluate_prompt(
        client=client,
        model=model,
        messages=messages,
        think=think,
        search=search,
        session_id=session_id,
        fresh_session=False,
        use_tools=use_tools
    )

# ─────────────────────────────────────────────────────────────────────────────
# ANTHROPIC COMPATIBLE INTERCEPTOR & REVERSE PROXY (PHASE 3 & 4)
# ─────────────────────────────────────────────────────────────────────────────

app = FastAPI()

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.api_route("/", methods=["GET", "HEAD"], include_in_schema=False)
async def root_probe():
    return Response(status_code=200)

@app.get("/v1/models")
async def get_models():
    return {
        "object": "list",
        "data": [
            {"id": "claude-3-7-sonnet", "object": "model", "owned_by": "anthropic"},
            {"id": "claude-3-5-sonnet-20241022", "object": "model", "owned_by": "anthropic"}
        ]
    }

def translate_anthropic_to_openai_messages(anthropic_msgs: list) -> list:
    """Translates incoming Anthropic messages structures to standard OpenAI format."""
    openai_msgs = []
    for msg in anthropic_msgs:
        role = msg.get("role")
        content = msg.get("content")

        if isinstance(content, list):
            text_parts = []
            tool_calls = []

            for part in content:
                part_type = part.get("type")
                if part_type == "text":
                    text_parts.append(part.get("text", ""))
                elif part_type == "tool_use":
                    tool_calls.append({
                        "id": part.get("id"),
                        "type": "function",
                        "function": {
                            "name": part.get("name"),
                            "arguments": json.dumps(part.get("input", {}))
                        }
                    })
                elif part_type == "tool_result":
                    tool_result_content = part.get("content", "")
                    if isinstance(tool_result_content, list):
                        tool_result_content = "\n".join([str(item) for item in tool_result_content])
                    openai_msgs.append({
                        "role": "tool",
                        "tool_call_id": part.get("tool_use_id"),
                        "content": str(tool_result_content)
                    })

            if text_parts or tool_calls:
                new_msg = {
                    "role": role,
                    "content": "\n".join(text_parts) if text_parts else ""
                }
                if tool_calls:
                    new_msg["tool_calls"] = tool_calls
                openai_msgs.append(new_msg)
        else:
            openai_msgs.append({
                "role": role,
                "content": str(content) if content is not None else ""
            })
    return openai_msgs

def translate_anthropic_tools_to_openai(anthropic_tools: list) -> list:
    """Translates incoming Anthropic tool schemas to standard OpenAI format."""
    openai_tools = []
    for t in anthropic_tools:
        openai_tools.append({
            "type": "function",
            "function": {
                "name": t.get("name"),
                "description": t.get("description", ""),
                "parameters": t.get("input_schema", {})
            }
        })
    return openai_tools

def _sse(event: str, data: dict) -> str:
    """Build a single valid Server-Sent-Event frame: 'event: <e>\\ndata: <json>\\n\\n'."""
    return f"event: {event}\ndata: {json.dumps(data, ensure_ascii=False)}\n\n"


async def stream_openai_to_anthropic(
    upstream_url: str,
    payload: dict,
    session_id: str,
    fresh_session: bool,
    requested_model: str = "claude-3-5-sonnet-20241022",
) -> AsyncGenerator[str, None]:
    """Proxy an OpenAI-compatible stream to Anthropic-style SSE events.

    zai-proxy emits well-formed *incremental* OpenAI SSE: an empty init chunk,
    text deltas, tool_calls deltas (id/name then arguments), keep-alive chunks,
    and a final chunk carrying finish_reason ("stop" or "tool_calls").
    """
    log_verbose(f"Forwarding translated payload to {upstream_url}", payload)

    async def _emit(event: str, data: dict):
        if VERBOSE:
            if event == "content_block_delta":
                d = data.get("delta", {})
                summary = d.get("type", "")
                if d.get("type") == "text_delta":
                    summary += f" {len(d.get('text', ''))}ch"
                elif d.get("type") == "input_json_delta":
                    summary += f" {len(d.get('partial_json', ''))}ch"
                log_verbose(f"[EMIT] {event} #{data.get('index')} ({summary})")
            else:
                log_verbose(f"[EMIT] {event} #{data.get('index', '')}")
        yield _sse(event, data)

    async def _emit_text(text: str):
        """Emit a single complete text block (used for error reporting)."""
        async for f in _emit("message_start", {
            "type": "message_start",
            "message": {
                "id": f"msg_{generateID()[:16]}",
                "type": "message",
                "role": "assistant",
                "content": [],
                "model": requested_model,
                "stop_reason": None,
                "stop_sequence": None,
                "usage": {"input_tokens": 0, "output_tokens": 0},
            },
        }):
            yield f
        async for f in _emit("content_block_start", {
            "type": "content_block_start",
            "index": 0,
            "content_block": {"type": "text", "text": ""},
        }):
            yield f
        async for f in _emit("content_block_delta", {
            "type": "content_block_delta",
            "index": 0,
            "delta": {"type": "text_delta", "text": text},
        }):
            yield f
        async for f in _emit("content_block_stop", {
            "type": "content_block_stop",
            "index": 0,
        }):
            yield f
        async for f in _emit("message_delta", {
            "type": "message_delta",
            "delta": {"stop_reason": "end_turn", "stop_sequence": None},
            "usage": {"input_tokens": 0, "output_tokens": 0},
        }):
            yield f
        async for f in _emit("message_stop", {"type": "message_stop"}):
            yield f

    timeout = httpx.Timeout(120.0, connect=15.0)
    try:
        async with httpx.AsyncClient(timeout=timeout) as client:
            # Use the configured upstream credential for zai-proxy. NEVER forward
            # Claude Code's Anthropic Authorization header here: it is a separate
            # credential, and zai-proxy requires its own token (config.Auth.Token,
            # "Waguri" by default). Forwarding the wrong token yields HTTP 401.
            auth_header = f"Bearer {UPSTREAM_API_KEY}"
            request_headers = {
                "Authorization": auth_header,
                "Content-Type": "application/json",
                "X-Session-Id": session_id,
                "X-Fresh-Session": "true" if fresh_session else "false",
            }

            try:
                stream_ctx = client.stream(
                    "POST", upstream_url, headers=request_headers, json=payload
                )
                response = await stream_ctx.__aenter__()
            except httpx.HTTPError as exc:
                err_msg = (
                    f"Failed to reach upstream bridge at {upstream_url}: {exc}. "
                    f"Is zai-proxy running and listening?"
                )
                log_verbose(err_msg)
                async for f in _emit_text(err_msg):
                    yield f
                return

            if response.status_code != 200:
                err_text = await response.aread()
                err_msg = f"Upstream Bridge Error ({response.status_code}): {err_text.decode('utf-8', 'ignore')}"
                log_verbose(err_msg)
                await stream_ctx.__aexit__(None, None, None)
                async for f in _emit_text(err_msg):
                    yield f
                return

            message_id = f"msg_{generateID()[:16]}"
            async for f in _emit("message_start", {
                "type": "message_start",
                "message": {
                    "id": message_id,
                    "type": "message",
                    "role": "assistant",
                    "content": [],
                    "model": requested_model,
                    "stop_reason": None,
                    "stop_sequence": None,
                    "usage": {"input_tokens": 0, "output_tokens": 0},
                },
            }):
                yield f

            text_index = None
            text_started = False
            text_stopped = False
            tool_indices = {}          # OpenAI tool_call index -> Anthropic block index
            next_block_index = 0
            finish_reason = None
            saw_tool_use = False
            chunk_count = 0

            async for raw_line in response.aiter_lines():
                line = raw_line.strip()
                if not line or not line.startswith("data: "):
                    continue

                raw_data = line[6:]
                if raw_data == "[DONE]":
                    break

                try:
                    chunk = json.loads(raw_data)
                except Exception:
                    continue

                # ── Full upstream visibility (compact) ──
                if VERBOSE:
                    choice = (chunk.get("choices") or [{}])[0]
                    delta = choice.get("delta", {}) or {}
                    has_text = bool(delta.get("content"))
                    has_tc = bool(delta.get("tool_calls"))
                    log_verbose(
                        f"[CHUNK {chunk_count + 1}] finish_reason={choice.get('finish_reason')} "
                        f"text={has_text} tool_calls={has_tc}"
                    )
                    chunk_count += 1

                if "error" in chunk:
                    err_msg = chunk["error"].get("message", "Z.AI Upstream Error")
                    log_verbose(f"[UPSTREAM ERROR] {err_msg}")
                    async for f in _emit_text(f"Upstream error: {err_msg}"):
                        yield f
                    await stream_ctx.__aexit__(None, None, None)
                    return

                if "choices" not in chunk or len(chunk["choices"]) == 0:
                    continue

                choice = chunk["choices"][0]
                delta = choice.get("delta", {}) or {}
                if choice.get("finish_reason") is not None:
                    finish_reason = choice.get("finish_reason")

                # ---- Tool calls ----
                if delta.get("tool_calls"):
                    for tc in delta["tool_calls"]:
                        tc_idx = tc.get("index", 0)
                        if tc_idx not in tool_indices:
                            # Close any open text block before opening a tool block.
                            if text_started and not text_stopped:
                                async for f in _emit("content_block_stop", {
                                    "type": "content_block_stop",
                                    "index": text_index,
                                }):
                                    yield f
                                text_stopped = True
                            anthropic_idx = next_block_index
                            next_block_index += 1
                            tool_indices[tc_idx] = anthropic_idx
                            func = tc.get("function", {}) or {}
                            name = func.get("name", "") or ""
                            if not name:
                                log_verbose("[WARN] tool_use block has empty name; emitting anyway")
                            tool_id = tc.get("id") or f"tool_{tc_idx}"
                            async for f in _emit("content_block_start", {
                                "type": "content_block_start",
                                "index": anthropic_idx,
                                "content_block": {
                                    "type": "tool_use",
                                    "id": tool_id,
                                    "name": name,
                                    "input": {},
                                },
                            }):
                                yield f
                            saw_tool_use = True
                        anthropic_idx = tool_indices[tc_idx]
                        func = tc.get("function", {}) or {}
                        raw_args = func.get("arguments")
                        if raw_args:
                            # zai-proxy sends arguments as a JSON *string*; accept a dict too.
                            if isinstance(raw_args, dict):
                                partial = json.dumps(raw_args, ensure_ascii=False)
                            else:
                                partial = str(raw_args)
                            async for f in _emit("content_block_delta", {
                                "type": "content_block_delta",
                                "index": anthropic_idx,
                                "delta": {
                                    "type": "input_json_delta",
                                    "partial_json": partial,
                                },
                            }):
                                yield f
                    continue

                # ---- Text content ----
                content = delta.get("content", "")
                if content:
                    if not text_started:
                        async for f in _emit("content_block_start", {
                            "type": "content_block_start",
                            "index": next_block_index,
                            "content_block": {"type": "text", "text": ""},
                        }):
                            yield f
                        text_index = next_block_index
                        text_started = True
                        next_block_index += 1
                    async for f in _emit("content_block_delta", {
                        "type": "content_block_delta",
                        "index": text_index,
                        "delta": {"type": "text_delta", "text": str(content)},
                    }):
                        yield f

            # Close any still-open blocks in index order.
            if text_started and not text_stopped:
                async for f in _emit("content_block_stop", {
                    "type": "content_block_stop",
                    "index": text_index,
                }):
                    yield f
            for anthropic_idx in sorted(tool_indices.values()):
                async for f in _emit("content_block_stop", {
                    "type": "content_block_stop",
                    "index": anthropic_idx,
                }):
                    yield f

            if finish_reason == "tool_calls":
                saw_tool_use = True
            stop_reason = "tool_use" if saw_tool_use else "end_turn"

            if VERBOSE:
                log_verbose(
                    f"[DONE] blocks=text:{text_started} tools:{len(tool_indices)} "
                    f"finish_reason={finish_reason} -> stop_reason={stop_reason}"
                )

            async for f in _emit("message_delta", {
                "type": "message_delta",
                "delta": {"stop_reason": stop_reason, "stop_sequence": None},
                "usage": {"input_tokens": 0, "output_tokens": 0},
            }):
                yield f
            async for f in _emit("message_stop", {"type": "message_stop"}):
                yield f
            await stream_ctx.__aexit__(None, None, None)
    except httpx.HTTPError as exc:
        err_msg = (
            f"Failed to reach upstream bridge at {upstream_url}: {exc}. "
            f"Is zai-proxy running and listening?"
        )
        log_verbose(err_msg)
        async for f in _emit_text(err_msg):
            yield f


@app.post("/v1/messages")
async def anthropic_messages(request: Request):
    """Primary router mapping Claude Code's Anthropic requests to standard OpenAI."""
    odata = await request.json()
    headers = request.headers
    
    # Fix: web_search is a CLI-only feature, not available in API server mode
    web_search = False

    # 1. Resolve Session Thread Parameters
    session_id = headers.get("X-Session-Id", "zai-session-cli-default")
    fresh_session = headers.get("X-Fresh-Session", "false") == "true"

    # 2. Extract and Remap System Context & Messages
    system_prompt = odata.get("system", "")
    openai_msgs = translate_anthropic_to_openai_messages(odata.get("messages", []))
    if system_prompt:
        openai_msgs.insert(0, {"role": "system", "content": system_prompt})

    # 3. Resolve Model Mappings
    requested_model = odata.get("model") or "glm-5.2"
    mapped_model = requested_model
    if requested_model in {"claude-3-7-sonnet", "claude-3-5-sonnet-20241022"}:
        mapped_model = "glm-5.2"

    # 4. Resolve Thinking State Parameters
    think_enabled = False
    thinking_conf = odata.get("thinking", {})
    if thinking_conf and thinking_conf.get("type") == "enabled":
        think_enabled = True

    # 5. Resolve Tool schemas (from Claude Code) and add our websearch tool if enabled
    openai_tools = []
    if "tools" in odata:
        openai_tools = translate_anthropic_tools_to_openai(odata["tools"])

    # Add our websearch tool when web_search is enabled
    if web_search:
        # Find the websearch tool from our schema
        for tool in TOOLS_SCHEMA:
            if tool["function"]["name"] == "websearch":
                openai_tools.append(tool)
                break

    # 6. Construct unified OpenAI request payload
    openai_payload = {
        "model": mapped_model,
        "messages": openai_msgs,
        "stream": True,
        "deepThink": think_enabled,
        "webSearch": web_search  # Use the web_search flag directly
    }
    if openai_tools:
        openai_payload["tools"] = openai_tools
        openai_payload["tool_choice"] = "auto"

    log_verbose("Incoming Anthropic /v1/messages request", {
        "session_id": session_id,
        "fresh_session": fresh_session,
        "model": requested_model,
        "thinking_enabled": think_enabled,
        "message_count": len(odata.get("messages", [])),
        "tool_count": len(odata.get("tools", [])),
        "messages": odata.get("messages", []),
        "tools": odata.get("tools", []),
    })
    log_verbose("Translated OpenAI payload", openai_payload)

    upstream_completions_url = f"{UPSTREAM_BASE_URL}/chat/completions"

    return StreamingResponse(
        stream_openai_to_anthropic(
            upstream_completions_url,
            openai_payload,
            session_id,
            fresh_session,
            requested_model,
        ),
        media_type="text/event-stream"
    )

def run_fastapi_server(port: int, verbose: bool = False):
    """Background runner for FastAPI reverse proxy."""
    global VERBOSE
    VERBOSE = verbose
    log_config = LOGGING_CONFIG
    log_config["formatters"]["default"]["fmt"] = "%(asctime)s - %(levelname)s - %(message)s"
    uvicorn.run(app, host="127.0.0.1", port=port, log_level="info" if verbose else "warning", access_log=verbose)

# ─────────────────────────────────────────────────────────────────────────────
# MAIN CLIENT & SERVER RUNNER
# ─────────────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Z.AI Direct Bridge CLI Agent Evaluator")
    parser.add_argument("--model", "-m", type=str, help="Skip interactive menu and set target model directly")
    parser.add_argument("--prompt", "-p", type=str, help="Execute a single prompt directly and exit immediately")
    parser.add_argument("--think", "-t", action="store_true", default=True, help="Enable Deep Think (default: True)")
    parser.add_argument("--no-think", dest="think", action="store_false", help="Disable Deep Think")
    parser.add_argument("--search", "-s", action="store_true", default=True, help="Enable Web Search (default: True)")
    parser.add_argument("--no-search", dest="search", action="store_false", help="Disable Web Search")
    parser.add_argument("--key", "-k", type=str, default=DEFAULT_API_KEY, help="API Key credentials (default: Waguri)")
    parser.add_argument("--url", "-u", type=str, default=DEFAULT_BASE_URL, help="Base API target URL")
    parser.add_argument("--session", "-S", type=str, default="zai-session-cli-default", help="Custom session thread ID")
    parser.add_argument("--fresh", "-F", action="store_true", default=False, help="Force a fresh thread initialization")
    parser.add_argument("--no-tools", action="store_true", default=False, help="Turn off OpenAI agent tool-calling capabilities")
    parser.add_argument("--port", "-P", type=int, default=8080, help="Local FastAPI proxy server port (default: 8080)")
    parser.add_argument("--server-only", action="store_true", default=False, help="Run only the FastAPI translator proxy server")
    parser.add_argument("--verbose", action="store_true", default=False, help="Print detailed Anthropic/OpenAI translation traces and upstream payloads")

    args = parser.parse_args()
    global VERBOSE, UPSTREAM_BASE_URL, UPSTREAM_API_KEY
    VERBOSE = args.verbose
    UPSTREAM_BASE_URL = args.url
    UPSTREAM_API_KEY = args.key

    # Check if port is already in use
    def is_port_in_use(port: int) -> bool:
        import socket
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            return s.connect_ex(('127.0.0.1', port)) == 0

    port_in_use = is_port_in_use(args.port)
    if port_in_use:
        console.print(f"[warning]⚠[/warning] Port {args.port} is already in use. Skipping proxy server startup.")
    else:
        # Launch FastAPI reverse-proxy server on port 8080 in a background daemon thread
        server_thread = threading.Thread(target=run_fastapi_server, args=(args.port, args.verbose), daemon=True)
        server_thread.start()

        # Pause a brief moment to let uvicorn initialize
        time.sleep(1.5)

    if args.server_only:
        console.print(f"\n[success]✓[/success] [bold green]FastAPI Anthropic -> OpenAI Reverse Proxy active![/bold green]")
        console.print(f"  Listening on: [cyan]http://localhost:{args.port}[/cyan]")
        console.print("  Press Ctrl+C to terminate.")
        try:
            while True:
                time.sleep(1)
        except KeyboardInterrupt:
            console.print("\n[info]Shutting down proxy server.[/info]")
            sys.exit(0)

    console.print(Panel.fit(
        "🔥 [bold cyan]Z.AI Direct Bridge CLI Agent Evaluator[/bold cyan] 🔥\n"
        "Powered by modernc SQLite and Native Go-Ciphers",
        border_style="cyan"
    ))

    if not run_health_check(args.url):
        sys.exit(1)

    # Initialize OpenAI Client matching target local server credentials
    client = OpenAI(base_url=args.url, api_key=args.key)

    # Fetch and select model
    models = get_available_models(client)
    
    # Resolve target model
    active_model = args.model if args.model in models else None
    if not active_model:
        active_model = select_model(models, default_model=args.model)

    deep_think = args.think
    web_search = args.search
    session_id = args.session
    fresh_trigger = args.fresh
    use_tools = not args.no_tools

    # Enable server-side history compilation for the active model
    enable_history_persistence(active_model, args.url, args.key)

    # ---------- Single Direct Prompt Mode ----------
    if args.prompt:
        messages = [{"role": "user", "content": args.prompt}]
        evaluate_prompt(
            client=client,
            model=active_model,
            messages=messages,
            think=deep_think,
            search=web_search,
            session_id=session_id,
            fresh_session=fresh_trigger,
            use_tools=use_tools
        )
        sys.exit(0)

    # ---------- Interactive Loop Mode ----------
    conversation_history = []

    console.print("\n[bold green]Shell is active![/bold green] Input prompts below.")
    console.print("💡 [meta]Commands Available:[/meta]")
    console.print("   [info]/think[/info]   - Toggle Deep Think status (current: [cyan]%s[/cyan])" % ("ON" if deep_think else "OFF"))
    console.print("   [info]/search[/info]  - Toggle Web Search status (current: [cyan]%s[/cyan])" % ("ON" if web_search else "OFF"))
    console.print("   [info]/model[/info]   - Change active model selection")
    console.print("   [info]/fresh[/info]   - Initialize a fresh session on the active thread")
    console.print("   [info]/clear[/info]   - Wipe local thread conversation history")
    console.print("   [info]/nuke[/info]    - Wipe ALL conversation histories globally on the bridge")
    console.print("   [info]/exit[/info]    - Terminate evaluator shell\n")

    while True:
        try:
            # Custom prompt bar showing configuration states
            prompt_indicator = f"({active_model} | Session: {session_id} | Think:{'ON' if deep_think else 'OFF'} | Search:{'ON' if web_search else 'OFF'}) >>> "
            user_input = input(prompt_indicator).strip()

            if not user_input:
                continue

            # Command Routing
            if user_input.lower() == "/exit":
                console.print("[info]Exiting evaluator shell. Goodbye![/info]")
                break

            elif user_input.lower() == "/think":
                deep_think = not deep_think
                console.print(f"[success]Deep Think toggled to:[/success] [bold cyan]{'ON' if deep_think else 'OFF'}[/bold cyan]")
                continue

            elif user_input.lower() == "/search":
                web_search = not web_search
                console.print(f"[success]Web Search toggled to:[/success] [bold cyan]{'ON' if web_search else 'OFF'}[/bold cyan]")
                console.print(f"[info]Web Search is {'ENABLED' if web_search else 'DISABLED'}. The model will use the websearch tool when needed.[/info]")
                continue

            elif user_input.lower() == "/model":
                active_model = select_model(models)
                enable_history_persistence(active_model, args.url, args.key)
                console.print(f"[success]Active model updated to:[/success] [bold cyan]{active_model}[/bold cyan]")
                continue

            elif user_input.lower() == "/fresh":
                fresh_trigger = True
                conversation_history.clear()
                console.print(f"[success]✓[/success] Fresh session status flagged for next message.")
                continue

            elif user_input.lower() == "/clear":
                clear_active_session(session_id, args.url, args.key)
                conversation_history.clear()
                fresh_trigger = True
                continue

            elif user_input.lower() == "/nuke":
                clear_all_sessions(args.url, args.key)
                conversation_history.clear()
                fresh_trigger = True
                continue

            # Append user message
            conversation_history.append({"role": "user", "content": user_input})

            # Evaluate Prompt
            evaluate_prompt(
                client=client, 
                model=active_model, 
                messages=conversation_history, 
                think=deep_think, 
                search=web_search,
                session_id=session_id,
                fresh_session=fresh_trigger,
                use_tools=use_tools
            )
            
            # Reset fresh trigger after the first execution
            if fresh_trigger:
                fresh_trigger = False

        except (KeyboardInterrupt, EOFError):
            console.print("\n[info]Exiting shell session.[/info]")
            break


if __name__ == "__main__":
    main()
