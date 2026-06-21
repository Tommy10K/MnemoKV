import {
  Bar,
  BarChart as RechartsBarChart,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"

export type BarRow = {
  name: string
  value: number
  color: string
}

type Props = {
  data: BarRow[]
  format?: (value: number) => string
  height?: number
  yLabel?: string
  ariaLabel: string
}

export function BarChart({ data, format, height = 320, yLabel, ariaLabel }: Props) {
  if (data.length === 0) {
    return (
      <div
        className="flex items-center justify-center rounded-md border border-dashed border-[#1f2937] text-sm text-[#8b949e]"
        style={{ height }}
      >
        no data
      </div>
    )
  }

  return (
    <div className="min-w-0" role="img" aria-label={ariaLabel} style={{ height }}>
      <ResponsiveContainer width="100%" height="100%" minWidth={0}>
        <RechartsBarChart data={data} margin={{ top: 8, right: 12, left: 0, bottom: 32 }}>
          <XAxis
            dataKey="name"
            tick={{ fill: "#9ca3af", fontSize: 11 }}
            angle={-25}
            textAnchor="end"
            interval={0}
            height={48}
          />
          <YAxis
            tick={{ fill: "#9ca3af", fontSize: 11 }}
            tickFormatter={(v) => (format ? format(Number(v)) : String(v))}
            width={70}
            label={
              yLabel
                ? { value: yLabel, angle: -90, position: "insideLeft", fill: "#8b949e", fontSize: 11 }
                : undefined
            }
          />
          <Tooltip
            cursor={{ fill: "#1f2937", opacity: 0.4 }}
            contentStyle={{
              background: "#0b0f17",
              border: "1px solid #1f2937",
              color: "#e6edf3",
              fontSize: 12,
            }}
            formatter={(value) => (format ? format(Number(value)) : value)}
          />
          <Bar dataKey="value" isAnimationActive={false}>
            {data.map((row) => (
              <Cell key={row.name} fill={row.color} />
            ))}
          </Bar>
        </RechartsBarChart>
      </ResponsiveContainer>
      <span className="sr-only">
        {data.map((row) => `${row.name}: ${format ? format(row.value) : row.value}`).join("; ")}
      </span>
    </div>
  )
}
