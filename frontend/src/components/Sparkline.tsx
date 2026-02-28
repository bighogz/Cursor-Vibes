import { memo } from "react";

interface Props {
  data: number[];
  positive: boolean;
  width?: number;
  height?: number;
}

export const Sparkline = memo(function Sparkline({
  data,
  positive,
  width = 80,
  height = 28,
}: Props) {
  if (data.length < 2) return null;

  const pad = 2;
  const min = Math.min(...data);
  const max = Math.max(...data);
  const range = max - min || 1;

  const points = data.map((v, i) => {
    const x = pad + (i / (data.length - 1)) * (width - pad * 2);
    const y = pad + (1 - (v - min) / range) * (height - pad * 2);
    return { x, y };
  });

  const linePath = points
    .map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(1)},${p.y.toFixed(1)}`)
    .join(" ");

  const lastPt = points[points.length - 1];
  const firstPt = points[0];
  const areaPath = `${linePath} L${lastPt.x.toFixed(1)},${height} L${firstPt.x.toFixed(1)},${height} Z`;

  const stroke = positive ? "#22c55e" : "#ef4444";
  const fill = positive ? "rgba(34,197,94,0.08)" : "rgba(239,68,68,0.08)";

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className="flex-shrink-0"
      aria-hidden="true"
    >
      <path d={areaPath} fill={fill} />
      <polyline
        points={points.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(" ")}
        fill="none"
        stroke={stroke}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
});
