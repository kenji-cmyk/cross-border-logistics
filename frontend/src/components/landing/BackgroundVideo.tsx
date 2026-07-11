import { useRef } from "react";
import { useReducedMotion } from "../../hooks/useReducedMotion";
import { useVideoFadeLoop } from "../../hooks/useVideoFadeLoop";

const VIDEO = "https://d8j0ntlcm91z4.cloudfront.net/user_38xzZboKViGWJOttwIXH07lWA1P/hf_20260328_083109_283f3553-e28f-428b-a723-d639c617eb2b.mp4";
export function BackgroundVideo() {
  const ref = useRef<HTMLVideoElement>(null);
  useVideoFadeLoop(ref, useReducedMotion());
  return <div aria-hidden="true" className="pointer-events-none absolute inset-0 z-0 overflow-hidden">
    <video ref={ref} src={VIDEO} autoPlay muted playsInline preload="metadata" className="absolute h-[calc(100%-300px)] w-full object-cover object-center grayscale-[20%] saturate-[80%]" style={{ top: "300px", insetInline: 0, bottom: 0, opacity: 0 }} />
    <div className="absolute inset-0 bg-gradient-to-b from-[#F7F9FC] via-[#F7F9FC]/20 to-[#F7F9FC]" />
    <div className="absolute inset-0 bg-gradient-to-r from-[#F7F9FC]/80 via-transparent to-[#F7F9FC]/60" />
  </div>;
}
