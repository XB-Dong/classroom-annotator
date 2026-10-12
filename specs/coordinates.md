# 坐标与抽帧规格

版本：1.0.0｜状态：已冻结，Astra审核通过

bbox=[x,y,width,height]，原图左上角原点，有限浮点数，x/y>=0，width/height>0，x+width<=image.width且y+height<=image.height。前端transform屏幕↔原图双向；缩放只改变显示。拖/resize结束后归一化并夹到图像边界，拒绝退化框。后端独立验证。新建手势宽或高小于2原图像素视为误触不创建；这个阈值仅用于新建手势，已有框/API仍按正宽高契约验证。部分遮挡标可见头部，完全不可见不标，只标学生。

抽帧区间[startSeconds,endSeconds)，时间输入只接受毫秒精度，intervalSeconds>=0.001秒，start默认0/end默认ceil(durationSeconds*1000)/1000。转整数毫秒后验证0<=startMs<endMs<=ceil(durationMs)；targetMs=startMs+n*intervalMs<endMs，预估数量用整数上取整，超过配置上限拒绝。FFmpeg每目标时间采用准确输出seek（输入后-ss），取第一个展示时间不早于目标的帧；尾部没有可解码帧时任务明确失败FRAME_UNAVAILABLE，不静默少抽或复制前帧。timestampMs表示采样目标，不表示源PTS，UI亦标明采样时间。每张JPEG核对实际尺寸，合成恒帧率视频验证目标时间和数量。

取消请求持久化后处于cancelling，子进程退出并清理未发布临时批次后才标cancelled；queued工作可立即取消。完成发布与取消请求通过事务状态比较解决竞争，先提交的终态获胜，后续取消返回现有终态。completed才公开完整帧集合。重抽保留原批次。
