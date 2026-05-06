type ScoreCardProps = {
  label: string;
  value: string;
  tone?: "lime" | "cyan" | "amber";
};

export function ScoreCard({ label, value, tone = "cyan" }: ScoreCardProps) {
  return (
    <div className="metric">
      <span className="label">{label}</span>
      <span className="value" style={{ color: `var(--${tone})` }}>
        {value}
      </span>
    </div>
  );
}
