FROM python:3.13-slim

LABEL org.opencontainers.image.title="PhosphorNet Python Door SDK"
LABEL org.opencontainers.image.description="Base image for Python stdio doors that speak the PhosphorNet runtime contract"

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV PYTHONPATH=/opt/phosphornet-sdk

WORKDIR /door

COPY phosphornet /opt/phosphornet-sdk/phosphornet

ENTRYPOINT ["python3", "-m", "phosphornet.runtime"]
