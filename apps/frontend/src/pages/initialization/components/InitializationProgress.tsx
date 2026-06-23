type InitializationProgressProps = {
  progress: number;
};

export function InitializationProgress(props: InitializationProgressProps) {
  const progress = Math.max(0, Math.min(100, props.progress));

  return (
    <section className="mt-4">
      <div className="mb-1.5 flex items-center justify-between">
        <span className="text-[14px] font-semibold text-[#374151]">初始化进度</span>
        <span className="text-[15px] font-bold text-[#1677ff]">{progress}%</span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-[#e5e7eb]" aria-label={`初始化进度 ${progress}%`}>
        <div className="h-full rounded-full bg-[#1677ff]" style={{ width: `${progress}%` }} />
      </div>
    </section>
  );
}
