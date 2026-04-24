# FunASR 录音文件识别 REST API 文档

> 来源：[阿里云 Model Studio FunASR 录音文件识别 RESTful API](https://help.aliyun.com/zh/model-studio/fun-asr-recorded-speech-recognition-restful-api?spm=a2c4g.11186623.0.i2)

## 1. API 地址

| 接口     | URL                                                                      | 方法 |
| -------- | ------------------------------------------------------------------------ | ---- |
| 提交任务 | `https://dashscope.aliyuncs.com/api/v1/services/audio/asr/transcription` | POST |
| 查询任务 | `https://dashscope.aliyuncs.com/api/v1/tasks/{task_id}`                  | GET  |

> 新加坡地域需使用 `https://dashscope-intl.aliyuncs.com/api/v1/services/audio/asr/transcription`

## 2. 请求头

### 提交任务请求头

| Header            | 值               |
| ----------------- | ---------------- |
| Authorization     | Bearer {api-key} |
| Content-Type      | application/json |
| X-DashScope-Async | enable           |

### 查询任务请求头

| Header        | 值               |
| ------------- | ---------------- |
| Authorization | Bearer {api-key} |
| Content-Type  | application/json |

## 3. 提交任务请求体

```json
{
    "model": "fun-asr",
    "input": {
        "file_urls": [
            "https://example.com/audio.wav"
        ]
    },
    "parameters": {
        "channel_id": [0],
        "diarization_enabled": false,
        "speaker_count": 2,
        "language_hints": ["zh", "en"]
    }
}
```

### 请求体参数说明

| 参数                           | 类型           | 必填 | 默认值 | 说明                                                                             |
| ------------------------------ | -------------- | ---- | ------ | -------------------------------------------------------------------------------- |
| model                          | string         | 是   | -      | 模型名：`fun-asr`、`fun-asr-2025-11-07`、`fun-asr-mtl`、`fun-asr-mtl-2025-08-25` |
| input.file_urls                | array[string]  | 是   | -      | 音频文件URL列表，最多100个，支持 HTTP/HTTPS                                      |
| input.vocabulary_id            | string         | 否   | -      | 热词ID，调用前需在控制台创建热词表并获取ID                                       |
| parameters.channel_id          | array[integer] | 否   | [0]    | 音轨索引，从0开始                                                                |
| parameters.diarization_enabled | boolean        | 否   | false  | 是否启用自动说话人分离，仅适用单声道                                             |
| parameters.speaker_count       | integer        | 否   | -      | 说话人数量参考值，范围2-100                                                      |
| parameters.language_hints      | array[string]  | 否   | -      | 语言提示，如 `["zh", "en"]`                                                      |
| parameters.special_word_filter | boolean        | 否   | false  | 是否启用特殊词汇过滤，过滤部分特殊符号或不规范内容                               |

### 支持的音频格式

aac、amr、avi、flac、flv、m4a、mkv、mov、mp3、mp4、mpeg、ogg、opus、wav、webm、wma、wmv

### 约束条件

- 音频大小：不超过 2GB
- 时长：12小时以内
- 批处理：单次最多100个文件URL

## 4. 提交任务响应

```json
{
    "output": {
        "task_status": "PENDING",
        "task_id": "c2e5d63b-96e1-4607-bb91-************"
    },
    "request_id": "77ae55ae-be17-97b8-9942-************"
}
```

### 响应字段说明

| 字段               | 类型   | 说明             |
| ------------------ | ------ | ---------------- |
| output.task_status | string | 任务状态         |
| output.task_id     | string | 任务ID，用于查询 |
| request_id         | string | 请求ID，用于排查 |

## 5. 查询任务响应

```json
{
    "request_id": "f9e1afad-94d3-997e-a83b-************",
    "output": {
        "task_id": "f86ec806-4d73-485f-a24f-************",
        "task_status": "SUCCEEDED",
        "submit_time": "2024-09-12 15:11:40.041",
        "scheduled_time": "2024-09-12 15:11:40.071",
        "end_time": "2024-09-12 15:11:40.903",
        "results": [
            {
                "file_url": "https://example.com/audio.wav",
                "transcription_url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/...",
                "subtask_status": "SUCCEEDED"
            }
        ],
        "task_metrics": {
            "TOTAL": 1,
            "SUCCEEDED": 1,
            "FAILED": 0
        }
    },
    "usage": {
        "duration": 9
    }
}
```

### 查询响应字段说明

| 字段                        | 类型   | 说明                           |
| --------------------------- | ------ | ------------------------------ |
| output.task_status          | string | 任务状态                       |
| output.task_id              | string | 任务ID                         |
| output.submit_time          | string | 提交时间                       |
| output.scheduled_time       | string | 调度时间                       |
| output.end_time             | string | 结束时间                       |
| output.results              | array  | 子任务结果列表                 |
| results[].file_url          | string | 原音频文件URL                  |
| results[].transcription_url | string | 识别结果下载链接，有效期24小时 |
| results[].subtask_status    | string | 子任务状态                     |
| results[].code              | string | 错误码（仅失败时存在）         |
| results[].message           | string | 错误信息（仅失败时存在）       |
| output.task_metrics         | object | 任务统计                       |
| task_metrics.TOTAL          | int    | 总数                           |
| task_metrics.SUCCEEDED      | int    | 成功数                         |
| task_metrics.FAILED         | int    | 失败数                         |
| usage.duration              | int    | 总时长（秒）                   |

## 6. 任务状态

| 状态      | 说明   |
| --------- | ------ |
| PENDING   | 排队中 |
| RUNNING   | 处理中 |
| SUCCEEDED | 成功   |
| FAILED    | 失败   |

> 当任务包含多个子任务时，只要存在任一子任务成功，整个任务状态将标记为 SUCCEEDED，需通过 `subtask_status` 字段判断具体子任务结果。

## 7. 识别结果格式（从 transcription_url 下载）

```json
{
    "file_url": "https://example.com/audio.wav",
    "properties": {
        "audio_format": "pcm_s16le",
        "channels": [0],
        "original_sampling_rate": 16000,
        "original_duration_in_milliseconds": 3834
    },
    "transcripts": [
        {
            "channel_id": 0,
            "content_duration_in_milliseconds": 3720,
            "text": "Hello world, 这里是阿里巴巴语音实验室。",
            "sentences": [
                {
                    "begin_time": 100,
                    "end_time": 3820,
                    "text": "Hello world, 这里是阿里巴巴语音实验室。",
                    "sentence_id": 1,
                    "speaker_id": 0,
                    "words": [
                        {
                            "begin_time": 100,
                            "end_time": 596,
                            "text": "Hello ",
                            "punctuation": ""
                        }
                    ]
                }
            ]
        }
    ]
}
```

### 识别结果字段说明

| 字段                                           | 类型    | 说明                           |
| ---------------------------------------------- | ------- | ------------------------------ |
| properties.audio_format                        | string  | 源音频格式                     |
| properties.channels                            | array   | 音轨索引                       |
| properties.original_sampling_rate              | integer | 采样率(Hz)                     |
| properties.original_duration_in_milliseconds   | integer | 原始时长(ms)                   |
| transcripts[].channel_id                       | int     | 音轨索引                       |
| transcripts[].content_duration_in_milliseconds | int     | 语音内容时长(ms)，用于计费     |
| transcripts[].text                             | string  | 完整文本                       |
| transcripts[].sentences[].begin_time           | int     | 句子开始时间(ms)               |
| transcripts[].sentences[].end_time             | int     | 句子结束时间(ms)               |
| transcripts[].sentences[].text                 | string  | 句子文本                       |
| transcripts[].sentences[].sentence_id          | int     | 句子ID                         |
| transcripts[].sentences[].speaker_id           | int     | 说话人ID（仅启用说话人分离时） |
| transcripts[].sentences[].words[].text         | string  | 词文本                         |
| transcripts[].sentences[].words[].punctuation  | string  | 预测标点                       |

## 8. 调用示例

### 8.1 cURL 提交任务

```bash
curl --location 'https://dashscope.aliyuncs.com/api/v1/services/audio/asr/transcription' \
     --header "Authorization: Bearer $DASHSCOPE_API_KEY" \
     --header "Content-Type: application/json" \
     --header "X-DashScope-Async: enable" \
     --data '{
         "model": "fun-asr",
         "input": {
             "file_urls": [
                 "https://dashscope.oss-cn-beijing.aliyuncs.com/samples/audio/paraformer/hello_world_female2.wav"
             ]
         },
         "parameters": {
             "channel_id": [0]
         }
     }'
```

### 8.2 cURL 查询任务

```bash
curl --location 'https://dashscope.aliyuncs.com/api/v1/tasks/{task_id}' \
     --header "Authorization: Bearer $DASHSCOPE_API_KEY"
```

## 9. 支持语言

`fun-asr`、`fun-asr-2025-11-07`、`fun-asr-mtl` 支持：

zh(中文)、en(英文)、ja(日语)、ko(韩语)、vi(越南语)、th(泰语)、id(印尼语)、ms(马来语)、tl(菲律宾语)、hi(印地语)、ar(阿拉伯语)、fr(法语)、de(德语)、es(西班牙语)、pt(葡萄牙语)、ru(俄语)、it(意大利语)、nl(荷兰语)、sv(瑞典语)、da(丹麦语)、fi(芬兰语)、no(挪威语)、el(希腊语)、pl(波兰语)、cs(捷克语)、hu(匈牙利语)、ro(罗马尼亚语)、bg(保加利亚语)、hr(克罗地亚语)、sk(斯洛伐克语)
