from prometheus_client import Counter, Histogram


class Metrics:
    def __init__(self) -> None:
        self.messages_consumed = Counter(
            "detector_messages_consumed_total",
            "Total messages polled from Kafka",
        )
        self.consumer_errors = Counter(
            "detector_consumer_errors_total",
            "Kafka consumer-level errors",
        )
        self.parse_errors = Counter(
            "detector_parse_errors_total",
            "Messages that failed JSON parsing",
        )
        self.windows_evaluated = Counter(
            "detector_windows_evaluated_total",
            "Evaluation ticks completed",
        )
        self.alerts_fired = Counter(
            "detector_alerts_fired_total",
            "Alerts dispatched, labelled by rule and severity",
            ["rule", "severity"],
        )
        self.evaluation_latency = Histogram(
            "detector_evaluation_latency_seconds",
            "Time spent inside a single evaluation tick",
        )
        self.dispatcher_errors = Counter(
            "detector_dispatcher_errors_total",
            "Dispatcher failures labelled by destination",
            ["destination"],
        )
