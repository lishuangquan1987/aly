package io.github.lishuangquan1987.alyclient;

/** 取消令牌；生命周期停止时返回 true。 */
@FunctionalInterface
public interface CancelToken {
    CancelToken NEVER = new CancelToken() {
        @Override
        public boolean isCancelled() {
            return false;
        }
    };

    boolean isCancelled();
}