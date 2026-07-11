export function LogisticsRouteGraphic() {
  return <svg aria-hidden="true" viewBox="0 0 1400 700" className="pointer-events-none absolute inset-x-0 top-48 z-[1] hidden h-[650px] w-full opacity-[.16] md:block">
    <path d="M90 460 C 340 180, 690 625, 1305 260" fill="none" stroke="#2563EB" strokeWidth="1.5" strokeDasharray="8 10" className="route-motion" />
    <path d="M90 460 C 340 180, 690 625, 1305 260" fill="none" stroke="#0B1220" strokeWidth=".5" opacity=".25" />
    {[[90,460],[430,340],[780,430],[1080,338],[1305,260]].map(([x,y], index) => <g key={x}><circle cx={x} cy={y} r={index === 4 ? 8 : 5} fill="#F7F9FC" stroke={index === 4 ? "#12B76A" : "#2563EB"} strokeWidth="2" />{index > 0 && index < 4 && <rect x={x-5} y={y-5} width="10" height="10" rx="2" fill="#0B1220" opacity=".5" />}</g>)}
  </svg>;
}
