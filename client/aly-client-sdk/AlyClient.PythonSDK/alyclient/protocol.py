from __future__ import annotations

from dataclasses import dataclass
from typing import Generic, Optional, TypeVar

T = TypeVar("T")


@dataclass
class Response(Generic[T]):
    """所有命令的统一响应。业务失败不会抛异常：is_success=False 时看 error_msg。

    reason 是 download_update 失败终态的原因机器码（协议 #13）：
    already_latest / partial_failure / materialize_failed / version_state_failed。
    机器判定优先于 error_msg 文案匹配。
    """

    is_success: bool
    error_msg: str
    data: Optional[T] = None
    reason: Optional[str] = None


def ok(data: T = None) -> Response[T]:
    return Response(is_success=True, error_msg="", data=data)


def ng(error_msg: str, reason: Optional[str] = None) -> Response[T]:
    return Response(is_success=False, error_msg=error_msg, data=None, reason=reason)