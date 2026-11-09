#!/usr/bin/env python3
"""T2.3b live evidence run: the T0.6 hybrid legs reproduced with Elastic-native
embeddings (.jina-embeddings-v3, 1024 dims) on the real deployment.

Reads ELASTIC_URL and LAMBO_ES_API_KEY from the environment (never prints
either). All HTTP goes through curl (one call per request; curl does not pay
the AAAA stall this network's stub resolver imposes on fresh getaddrinfo).

Usage: live.py <index> <out-dir>
  <index>   throwaway index name, deleted at the end of the run
  <out-dir> raw responses land here (git-ignored); stdout is the transcript

Exit 0 only if every assertion holds AND the index is deleted AND no
t23b* index remains.
"""
import json
import math
import os
import subprocess
import sys

DOC_IDS = ["p1", "p2", "p3", "p4", "p5", "r1", "r2", "p6"]
SKU_QUERY = "Brand X oxford shirt"
REASON_QUERY = "snug across the chest, pulls at the buttons"
SAME_LINE = ["p1", "p2", "p3", "p6"]
DISTRACTORS = ["p4", "p5"]
FAILURES = []


def check(name, cond, detail=""):
    print(("PASS " if cond else "FAIL ") + name + ((" -- " + str(detail)) if detail and not cond else ""))
    if not cond:
        FAILURES.append(name)


def curl(method, path, body=None, timeout=120):
    env = {k: v for k, v in os.environ.items()}
    cmd = ["curl", "-s", "-m", str(timeout), "-w", "\n%{http_code}",
           "-H", "Authorization: ApiKey " + env["LAMBO_ES_API_KEY"]]
    if body is not None:
        cmd += ["-H", "Content-Type: application/json", "-d", json.dumps(body)]
    if method != "GET":
        cmd += ["-X", method]
    cmd.append(env["ELASTIC_URL"] + path)
    out = subprocess.run(cmd, capture_output=True, text=True, check=True).stdout
    payload, _, code = out.rpartition("\n")
    return int(code.strip()), payload


def must(method, path, body=None, want=200):
    code, payload = curl(method, path, body)
    if code != want:
        print("FAIL http %s %s -> %d, want %d\n%s" % (method, path, code, want, payload[:500]))
        sys.exit(1)
    try:
        return json.loads(payload) if payload.strip() else {}
    except ValueError:
        print("FAIL http %s %s -> non-JSON body" % (method, path))
        sys.exit(1)


def embed(texts, input_type):
    d = must("POST", "/_inference/text_embedding/.jina-embeddings-v3",
             {"input": texts, "task_settings": {"input_type": input_type}})
    vecs = [row["embedding"] for row in d["text_embedding"]]
    assert all(len(v) == 1024 for v in vecs), "native width is not 1024"
    return vecs


def cosine(a, b):
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(x * x for x in b))
    return dot / (na * nb)


def rank_of(hits):
    return [h["_id"] for h in hits]


def above_before(order, higher, lower):
    rank = {doc_id: i for i, doc_id in enumerate(order)}
    return all(rank[h] < rank[lo] for h in higher for lo in lower
               if h in rank and lo in rank)


def main():
    index, outdir = sys.argv[1], sys.argv[2]
    os.makedirs(outdir, exist_ok=True)
    for var in ("ELASTIC_URL", "LAMBO_ES_API_KEY"):
        if not os.environ.get(var):
            print("FAIL missing from environment: %s (source tools/env.sh)" % var)
            sys.exit(1)

    repo = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..")
    docs = {d["doc_id"]: d["text"] for d in
            json.load(open(os.path.join(repo, "testdata", "elastic", "hybrid-documents.json")))["documents"]}

    # 1. Throwaway index, Lambo's pinned shape (plain float HNSW, cosine).
    must("PUT", "/" + index, {"mappings": {"properties": {
        "text": {"type": "text"},
        "vec": {"type": "dense_vector", "dims": 1024, "index": True, "similarity": "cosine"}}}},
        want=200)
    print("PASS index created: <throwaway>")

    # 2. Native INGEST vectors for the 8 T0.6 documents, one batched call.
    vecs = embed([docs[doc_id] for doc_id in DOC_IDS], "ingest")
    print("PASS 8 native INGEST vectors at 1024 dims")
    bulk = ""
    for doc_id, text, vec in zip(DOC_IDS, [docs[d] for d in DOC_IDS], vecs):
        bulk += json.dumps({"index": {"_index": index, "_id": doc_id}}) + "\n"
        bulk += json.dumps({"text": text, "vec": vec}) + "\n"
    env = os.environ
    proc = subprocess.run(
        ["curl", "-s", "-m", "120", "-H", "Authorization: ApiKey " + env["LAMBO_ES_API_KEY"],
         "-H", "Content-Type: application/x-ndjson", "-X", "POST",
         env["ELASTIC_URL"] + "/" + index + "/_bulk", "--data-binary", "@-"],
        input=bulk, capture_output=True, text=True, check=True)
    errors = json.loads(proc.stdout).get("errors", True)
    check("bulk index 8 docs", errors is False)
    must("POST", "/" + index + "/_refresh")

    # 3. BM25 leg: a new SKU-shaped text finds the returned line.
    d = must("POST", "/" + index + "/_search",
             {"query": {"match": {"text": SKU_QUERY}}, "size": 8})
    order = rank_of(d["hits"]["hits"])
    print("bm25 order:", " ".join("%s" % i for i in order))
    check("bm25: same-line shirts above distractors", above_before(order, SAME_LINE, DISTRACTORS), order)

    # 4. kNN leg over native vectors: fresh reason query, SEARCH input type.
    (qvec,) = embed([REASON_QUERY], "search")
    d = must("POST", "/" + index + "/_search",
             {"knn": {"field": "vec", "query_vector": qvec, "k": 8, "num_candidates": 100}, "size": 8})
    order = rank_of(d["hits"]["hits"])
    scores = {h["_id"]: round(h["_score"], 4) for h in d["hits"]["hits"]}
    print("knn order:", " ".join("%s:%s" % (i, scores[i]) for i in order))
    check("knn: reproduces recorded native shape r2/p1/r1", order[:3] == ["r2", "p1", "r1"], order)
    check("knn: returned line above the far distractor",
          above_before(order, ["r2", "r1", "p1", "p2", "p3"], ["p5"]), order)
    check("knn: roomier cut trails last", order[-1] == "p6", order)

    # 5. Combined leg: retriever RRF (top-level rank:rrf is refused here).
    d = must("POST", "/" + index + "/_search", {
        "retriever": {"rrf": {"retrievers": [
            {"standard": {"query": {"match": {"text": REASON_QUERY.split(",")[0]}}}},
            {"knn": {"field": "vec", "query_vector": qvec, "k": 8, "num_candidates": 100}}]}},
        "size": 8})
    order = rank_of(d["hits"]["hits"])
    print("rrf order:", " ".join("%s" % i for i in order))
    check("rrf: reason phrases in top 3", set(order[:3]) >= {"r2", "r1"}, order)
    check("rrf: distractors rank last", set(order[-2:]) == set(DISTRACTORS), order)

    # 6. Paraphrase pair, exact cosine, INGEST/INGEST (the Lambo configuration).
    a, b = embed([docs["r1"], docs["r2"]], "ingest")
    pair = cosine(a, b)
    print("paraphrase cosine r1/r2 (native INGEST/INGEST): %.4f (T0.6b 0.6769, settled 0.68)" % pair)
    check("paraphrase pair in paraphrase band", 0.60 <= pair <= 0.78, round(pair, 4))

    # 7. Delete the throwaway index and prove nothing t23b* remains.
    must("DELETE", "/" + index)
    code, payload = curl("GET", "/_cat/indices/t23b*?format=json")
    remaining = json.loads(payload) if code == 200 else "UNEXPECTED:%d" % code
    check("throwaway index deleted, no t23b* remains", remaining == [], remaining)
    code, payload = curl("GET", "/_cat/indices?format=json&h=index")
    print("indices now:", json.loads(payload) if code == 200 else "UNEXPECTED:%d" % code)

    if FAILURES:
        print("FAILURES: %s" % FAILURES)
        sys.exit(1)
    print("ALL LIVE LEGS PASS")


if __name__ == "__main__":
    main()
