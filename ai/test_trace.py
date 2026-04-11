"""Quick diagnostic to verify LangSmith tracing works end-to-end."""

import os
import sys


def main():
    print("=== LangSmith Tracing Diagnostic ===\n")

    api_key = os.environ.get("LANGSMITH_API_KEY", "")
    tracing = os.environ.get("LANGSMITH_TRACING", "")
    endpoint = os.environ.get("LANGSMITH_ENDPOINT", "")
    project = os.environ.get("LANGSMITH_PROJECT", "")

    print(f"LANGSMITH_API_KEY: {'set (' + api_key[:10] + '...)' if api_key else 'NOT SET'}")
    print(f"LANGSMITH_TRACING: {tracing or 'NOT SET'}")
    print(f"LANGSMITH_ENDPOINT: {endpoint or 'NOT SET'}")
    print(f"LANGSMITH_PROJECT: {project or 'NOT SET'}")

    if not api_key or not tracing:
        print("\n[FAIL] Required env vars not set. Run this in the same shell as uvicorn.")
        sys.exit(1)

    from langsmith import utils as ls_utils

    print(f"\nSDK tracing_is_enabled(): {ls_utils.tracing_is_enabled()}")

    if not ls_utils.tracing_is_enabled():
        print("[FAIL] SDK says tracing is disabled despite env vars being set.")
        sys.exit(1)

    from langsmith import Client

    client = Client()
    print(f"SDK endpoint: {client.api_url}")
    print(f"SDK project: {os.environ.get('LANGSMITH_PROJECT', 'default')}")

    print("\nSending a test LangChain call (ChatOllama)...")
    from langchain_core.prompts import ChatPromptTemplate
    from langchain_ollama import ChatOllama

    llm = ChatOllama(model="qwen3.5", temperature=0, format="json")
    prompt = ChatPromptTemplate.from_messages(
        [("system", "Return valid JSON: {{\"test\": true}}"), ("human", "ping")]
    )
    chain = prompt | llm
    result = chain.invoke({})
    content = result.content if hasattr(result, "content") else str(result)
    print(f"Model response: {content[:200]}")
    print("\n[OK] Request completed. Check LangSmith Runs tab for a new trace.")
    print("     It may take 10-30 seconds for the trace to appear.")


if __name__ == "__main__":
    main()
