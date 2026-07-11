import { useEffect, type RefObject } from "react";

export function calculateVideoOpacity(currentTime: number, duration: number, fadeSeconds = .5) {
  if (!Number.isFinite(currentTime) || !Number.isFinite(duration) || duration <= 0) return 0;
  if (currentTime < fadeSeconds) return Math.max(0, Math.min(1, currentTime / fadeSeconds));
  const remaining = duration - currentTime;
  if (remaining < fadeSeconds) return Math.max(0, Math.min(1, remaining / fadeSeconds));
  return 1;
}

export function useVideoFadeLoop(ref: RefObject<HTMLVideoElement | null>, reducedMotion: boolean) {
  useEffect(() => {
    const video = ref.current;
    if (!video) return;
    let frame = 0;
    let restartTimer: number | undefined;
    let disposed = false;
    const safePlay = () => { void video.play().catch(() => undefined); };
    if (reducedMotion) {
      video.style.opacity = ".46";
      video.pause();
      video.currentTime = 1;
      return;
    }
    const tick = () => {
      video.style.opacity = String(calculateVideoOpacity(video.currentTime, video.duration) * .46);
      frame = requestAnimationFrame(tick);
    };
    const ended = () => {
      video.style.opacity = "0";
      restartTimer = window.setTimeout(() => { if (!disposed) { video.currentTime = 0; safePlay(); } }, 100);
    };
    video.addEventListener("ended", ended);
    safePlay(); frame = requestAnimationFrame(tick);
    return () => { disposed = true; cancelAnimationFrame(frame); if (restartTimer) clearTimeout(restartTimer); video.removeEventListener("ended", ended); };
  }, [ref, reducedMotion]);
}
