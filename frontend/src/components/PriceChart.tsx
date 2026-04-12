import { memo } from "react";
import { fmtPrice } from "../lib/format";

interface Props {
  data: number[];
  positive: boolean;
  width?: number;
  height?: number;
}

export const PriceChart = memo(function PriceChart({
  data,
  positive,
  width = 340,
  height = 100,
}: Props) {
  if (data.length < 2) return null;

  const yAxisW = 52;
  const xAxisH = 18;
  const padT = 8;
  const padR = 4;
  const chartW = width - yAxisW - padR;
  const chartH = height - xAxisH - padT;

  const min = Math.min(...data);
  const max = Math.max(...data);
  const range = max - min || 1;

  const points = data.map((v, i) => ({
    x: yAxisW + (i / (data.length - 1)) * chartW,
    y: padT + (1 - (v - min) / range) * chartH,
  }));

  const linePath = points
    .map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(1)},${p.y.toFixed(1)}`)
    .join(" ");

  const lastPt = points[points.length - 1];
  const firstPt = points[0];
  const areaPath = `${linePath} L${lastPt.x.toFixed(1)},${padT + chartH} L${firstPt.x.toFixed(1)},${padT + chartH} Z`;

  const stroke = positive ? "#22c55e" : "#ef4444";
  const fill = positive ? "rgba(34,197,94,0.08)" : "rgba(239,68,68,0.08)";

  const yTicks = [max, (max + min) / 2, min];
  const gridColor = "rgba(255,255,255,0.06)";
  const textColor = "rgba(255,255,255,0.35)";

  const xLabels: { x: number; label: string }[] = [];
  const step = Math.max(1, Math.floor((data.length - 1) / 3));
  for (let i = 0; i <= data.length - 1; i += step) {
    xLabels.push({
      x: yAxisW + (i / (data.length - 1)) * chartW,
      label: i === 0 ? "1" : `${i + 1}`,
    });
  }
  if (xLabels[xLabels.length - 1]?.label !== `${data.length}`) {
    xLabels.push({
      x: yAxisW + chartW,
      label: `${data.length}`,
    });
  }

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className="flex-shrink-0"
    >
      {yTicks.map((tick, i) => {
        const y = padT + (1 - (tick - min) / range) * chartH;
        return (
          <g key={i}>
            <line
              x1={yAxisW}
              x2={yAxisW + chartW}
              y1={y}
              y2={y}
              stroke={gridColor}
              strokeDasharray="3,3"
            />
            <text
              x={yAxisW - 6}
              y={y + 3.5}
              textAnchor="end"
              fill={textColor}
              fontSize="9"
              fontFamily="ui-monospace, monospace"
            >
              {fmtPrice(tick)}
            </text>
          </g>
        );
      })}

      <path d={areaPath} fill={fill} />
      <polyline
        points={points.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(" ")}
        fill="none"
        stroke={stroke}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />

      {xLabels.map((lbl, i) => (
        <text
          key={i}
          x={lbl.x}
          y={padT + chartH + 13}
          textAnchor="middle"
          fill={textColor}
          fontSize="9"
          fontFamily="ui-monospace, monospace"
        >
          {lbl.label}
        </text>
      ))}
    </svg>
  );
});
