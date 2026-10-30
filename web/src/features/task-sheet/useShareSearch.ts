import { useEffect, useRef, useState } from "react";
import { searchShares } from "../../lib/api";
import type { SearchResult } from "../../lib/types";

export function useShareSearch(open: boolean, searchReady: boolean) {
  const [show, setShow] = useState(false);
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<SearchResult[]>([]);
  const [messages, setMessages] = useState<string[]>([]);
  const [searching, setSearching] = useState(false);
  const [previewHit, setPreviewHit] = useState<SearchResult | null>(null);
  const seq = useRef(0);

  useEffect(() => {
    if (!open) return;
    setShow(false);
    setQuery("");
    setHits([]);
    setMessages([]);
    setPreviewHit(null);
  }, [open]);

  useEffect(() => {
    if (!show) return;
    const current = (seq.current += 1);
    const keyword = query.trim();
    if (!searchReady || !keyword) {
      setHits([]);
      setMessages([]);
      setSearching(false);
      return;
    }
    const timer = window.setTimeout(() => {
      setSearching(true);
      searchShares(keyword)
        .then((result) => {
          if (seq.current !== current) return;
          setHits(result.results);
          setMessages(result.messages);
        })
        .catch((cause: unknown) => {
          if (seq.current !== current) return;
          setHits([]);
          setMessages([cause instanceof Error ? cause.message : "搜索失败"]);
        })
        .finally(() => {
          if (seq.current === current) setSearching(false);
        });
    }, 350);
    return () => window.clearTimeout(timer);
  }, [query, show, searchReady]);

  return {
    show,
    setShow,
    query,
    setQuery,
    hits,
    messages,
    searching,
    previewHit,
    setPreviewHit,
  };
}

export type ShareSearch = ReturnType<typeof useShareSearch>;
