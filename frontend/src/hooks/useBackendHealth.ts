import { useEffect, useState } from "react";
import { API_BASE_URL } from "../lib/api";

export type GatewayStatus = "checking" | "online" | "unavailable";
export function useBackendHealth() {
  const [status, setStatus] = useState<GatewayStatus>("checking");
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), 5_000);
    fetch(`${API_BASE_URL.replace(/\/$/, "")}/health`, { signal: controller.signal })
      .then((response) => setStatus(response.ok ? "online" : "unavailable"))
      .catch(() => setStatus("unavailable"))
      .finally(() => clearTimeout(timer));
    return () => { clearTimeout(timer); controller.abort(); };
  }, []);
  return status;
}
