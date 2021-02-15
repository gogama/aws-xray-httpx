

TODO: Need to talk about how X-Ray will panic with
"failed to begin subsegment named 'foo.com': segment cannot be found." if there's
no parent segment in the request plan context.

    - Workaround, ensure your request context has an execution.
    - Configure X-Ray not to panic.

TODO: Should make an example Lambda function.
