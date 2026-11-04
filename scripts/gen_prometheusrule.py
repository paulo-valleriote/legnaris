#!/usr/bin/env python3
"""Regenerates deploy/k8s/observability/prometheusrule.yaml from deploy/prometheus/rules.yml."""
import pathlib

import yaml

ROOT = pathlib.Path(__file__).resolve().parent.parent
SOURCE = ROOT / "deploy/prometheus/rules.yml"
TARGET = ROOT / "deploy/k8s/observability/prometheusrule.yaml"

HEADER = """# GENERATED from deploy/prometheus/rules.yml - edit that file, then run
# `make k8s-rules`. TestK8sPrometheusRuleMatchesRulesFile fails on drift.
"""


class Dumper(yaml.SafeDumper):
    pass


def represent_str(dumper, value):
    style = "|" if "\n" in value else None
    return dumper.represent_scalar("tag:yaml.org,2002:str", value, style=style)


Dumper.add_representer(str, represent_str)


def main():
    rules = yaml.safe_load(SOURCE.read_text())
    doc = {
        "apiVersion": "monitoring.coreos.com/v1",
        "kind": "PrometheusRule",
        "metadata": {"name": "legnaris-email-pipeline"},
        "spec": {"groups": rules["groups"]},
    }
    TARGET.write_text(HEADER + yaml.dump(doc, Dumper=Dumper, sort_keys=False, width=1000))


if __name__ == "__main__":
    main()
