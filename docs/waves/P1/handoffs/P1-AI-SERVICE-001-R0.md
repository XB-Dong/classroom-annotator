# P1-AI-SERVICE-001 R0 handoff

- Root task: P1-AI-SERVICE-001; base: 822eb26160958037b09542dd26623b833c50db45; branch: p1/ai.
- Start: 2026-10-14T01:36:00+08:00; R0 probe completed before 02:00; slice deadline: 02:18; wave deadline: 08:30.
- Requested coding model: gpt-6.1-sol/high, actual provider/model/reasoning: unknown, context: self-contained worker dispatch. No capacity error or fallback observed.
- Scope: ai-service/**, deploy/ai.Dockerfile preparation, P1-AI handoffs, external runtime/p1-ai. No DB, GLM, cloud mutation, media upload, shared contract edits or persistent server.
- Repair count: no implementation repair round consumed; one analytic fixture had an incorrect landscape y center, corrected from190 to220 because independent padding arithmetic proves expected y140 requires center220. Raw failed log preserved.

## Physical artifact and provenance

Leader recovered existing evaluated artifact into external runtime. Independent check confirms3,221,705 bytes and SHA256 a5ece478fc918787258afccd0b313e54319f8f39aeb1ceca11f27dbd269e78bc. Actual ORT1.23.2 loads CPU input images float32[1,3,640,640], output float32[1,38,8400]. Head=7 is independent score channel11; Body=0 and Face=16 are not used. ORT one intra/inter thread, sequential execution, spinning disabled.

471 README explicitly licenses the model MIT; its class table identifies Head=7, and its LICENSE includes the2024 authors and2025 Katsuya Hyodo. HRFFA author README explicitly names this exact Nano filename as MIT-licensed YOLOv9 trained/exported with PINTO0309/YOLO. Current471 path commit d069263de93154dba4d41052cfcb055a3bb93fb4; historical455 link is obsolete. Full MIT notice is retained in ai-service/MODEL-LICENSE. Source copies and their hashes remain external runtime/p1-ai/provenance. No giant bundle fetched.

## Actual sample results

Private Windows sample inventory contains8 PNGs and1 MP4, no JPEG. Eight PNGs were converted to JPEG in memory. Neither private images nor raw inference arrays enter Git or GLM. Raw full-frame arrays/results/private overlays reside external runtime/p1-ai. Full-frame default.25/.50 counts:1,0,0,0,0,0,0,0; end-to-end preprocess/inference/decode64.6–69.8ms. RGB0–1 outperforms bounded RGB/BGR/0–255 input checks; no alternate model or lower threshold was used.

Leader authorized same-model full frame plus2x2 tiles with15% tile overlap and global NMS. Counts after global NMS:10,13,9,13,12,10,13,12; five-invocation times301.6–319.0ms. Actual sample01 overlay shows10 plausible heads and at least15 clearly visible omitted heads; no obvious non-head false positive among its10 candidates. Leader independently viewed the actual private overlay and approved large-image1280+ tiling. These are functional observations, not independent precision/recall, accuracy, timing savings or deployment performance.

## Red and pending Green

- Bundled Python3.12.14 venv created external runtime/p1-ai/venv; approved direct pins successfully installed, exact resolved transitives recorded external resolved-requirements.txt. No Torch/CUDA.
- Named .NET Local\CVTE_P1_BUILD held for install and short probes, released in finally; no server remains.
- `python -m pytest tests/test_coordinates.py tests/test_output_validation.py -q`: actual RED exit1,12 failures due missing preprocessing, log coordinates-red.log. Initial implementation run exit1,11 passed/1 failed (corrected analytic fixture), log coordinates-green.log; final Green pending.
- `python -m pytest tests/test_http.py -q`: actual RED exit1,7 failures from unimplemented HTTP boundary, log http-red.log; Green pending.
- `python runtime/p1-ai/probe.py`: exit0, actual artifact+8 image inference evidence r0-model-results.json/probe.log.
- `python runtime/p1-ai/input-probe.py`: exit0, bounded4 input variants input-probe.json.
- `python runtime/p1-ai/tiling-probe.py`: exit0,8 real images/5 invocations each, r0-tiling-results.json/tiling-probe.log.

Next: finish coherent bounded authenticated HTTP service, analytic tile coordinate/global NMS tests, real model/JPEG/PNG HTTP tests, locked dependency hashes and Docker preparation; no final delivery or release acceptance claimed by this R0 handoff.
