!image.png

## **Collecting log files**

1. Make sure you already have `logs` directory in the root of this repository and have `app.log` file created by the http service from previous exercise.
2. Create a file named `fluent-bit.conf` under `scripts/fluentbit` directory with the following content:

    ```
    [SERVICE]
        Flush     1
        Log_Level info
    
    [INPUT]
        Name  tail
        Path  /app/logs/app.log
        Tag   http-service
    
    [OUTPUT]
        name  stdout
        match *
    ```

   This configuration tells fluentbit to tail the file `/app/logs/app.log` in fluentbit container and send the logs to its container `stdout`. By configuring the `stdout` output plugin with `match *`, it will send logs coming from all input plugins to the container `stdout`.

   To learn more about the configuration, please read the following documentation:

    - **Fluentbit configuration**
    - **Fluentbit tail input**
    - **Fluentbit stdout output**
3. Add the `fluentbit` service to the `docker-compose.yml` file:

    ```
      fluentbit:
        image:fluent/fluent-bit:2.1.10
        ports:
          -"24224:24224"
          -"24224:24224/udp"
        volumes:
          -./scripts/fluentbit/fluent-bit.conf:/fluent-bit/etc/fluent-bit.conf
          -./logs:/app/logs
    ```

   This will mount the `fluent-bit.conf` file to the container and mount the `logs` directory to the container so that fluentbit can tail the `app.log` file.

4. Run fluentbit container by running:

    ```
    docker compose up -d
    ```

5. Tail logs from the `fluentbit` container by running:

    ```
    docker compose logs -f fluentbit
    ```

6. In another terminal, run the http server by `go run main.go` and hit the API multiple times. If your application writes logs to the file, you should see the logs are being collected by the `fluentbit` container in its container stdout.

   If you don’t see any logs from fluentbit container, make sure that your application is writing logs to the file. You can check the file `cat logs/app.log`.


## **Collecting Nginx container logs**

You have learned how to collect logs from a file. In this exercise, you will learn how to collect logs from a container’s `stdout` using fluentbit. To simplify the exercise, we will use nginx container as the source of the logs.

1. Add the `nginx` service to the `docker-compose.yml` file:

    ```
      nginx:
        image:nginx
        ports:
          -"80:80"
        logging:
          driver:fluentd
          options:
            tag:nginx
            fluentd-sub-second-precision:'true'
    ```

   One special thing about this configuration is the `logging` section. This section tells docker to send the logs of the `nginx` container to fluentbit by using `fluentd` log driver. The `tag` option tells fluentbit to use `nginx` as the tag for the logs. The `fluentd-sub-second-precision` option tells fluentbit to use sub-second precision for the timestamp of the logs.

2. Update `fluent-bit.conf` file under `scripts/fluentbit` directory with the following content:

    ```
    [INPUT]
        Name forward
        Listen 0.0.0.0
        port 24224
    ```

   This configuration tells fluentbit to listen to port `24224` and use `forward` as the input plugin. The `forward` input plugin is used to receive logs from other container. In this case, the `nginx` container will send its logs to fluentbit using the `forward` protocol.

   For more information about the `forward` input plugin, please read the **input forward documentation**.

3. restart the `fluentbit` and `nginx` container:

    ```
    docker compose restart fluentbit
    docker compose up -d nginx
    ```

4. In two different terminal, tail the logs of the `fluentbit` and `nginx` containers:

    ```
    docker compose logs -f fluentbit
    ```

    ```
    docker compose logs -f nginx
    ```

   Once you start the `nginx` container, you should see the logs of the `nginx` container in the `fluentbit` container’s logs.

   > *Please make sure that you see the `nginx` tag attached to every logs recorded in the `fluentbit` container’s logs.*
>
5. Try to access the nginx container by accessing `http://localhost:80` from your browser. You should see the access log of the nginx container in the `fluentbit` container’s logs.

Now you have seen how to stream logs from a container stdout to fluentbit. If you have a containerized application, you can use this technique to stream the logs to fluentbit.

You have seen how to collect logs from a container’s `stdout` and files using fluentbit. However, you can’t do much with the logs yet. You will need a log aggregation system to store and query the logs. In this section, you will learn about Grafana Loki, a log aggregation system chosen for this course.

## **What is Grafana Loki?**

Grafana Loki is an open-source log aggregation system that is designed to be highly scalable and efficient. It is part of the Grafana Labs ecosystem, which includes the popular Grafana dashboard and visualization platform. Loki focuses on collecting, indexing, and searching logs, making it easier for users to analyze and troubleshoot issues within their applications and systems.

For more information about Grafana Loki, please read the **official documentation**.

Some key features of Grafana Loki that we will explore on this course:

1. **Log Aggregation**: Loki collects log data from various sources, allowing users to centralize their logs in one location for easy management and analysis.
2. **Label-based Indexing**: Loki uses label-based indexing, similar to Prometheus, which enables efficient querying and filtering of log data based on specific labels or metadata. This approach helps in organizing and retrieving logs related to specific components or services.
3. **Integration with Grafana**: Being part of the Grafana ecosystem, Loki seamlessly integrates with Grafana dashboards. Users can visualize log data alongside other metrics and monitoring information, providing a comprehensive view of system health.

## **How grafana works with loki?**

Here is high level overview of how Grafana works with Loki:

!Grafana Loki.

Logs from your applications are collected by a log collector agent and sent to Loki. Loki will act as persistent storage for the logs. You can define the log retention, storage type, etc. Later, you can query the logs using LogQL and visualize the logs using Grafana and LogCLI.

Here is list of reading materials I recommend to read:

- **Grafana loki overview**
- **Sending data to loki**
- **Grafana loki query**

## **Setting up Loki**

In earlier module about Grafana, we intentionally skipped grafana data source part. Now, let’s take a step back to add loki as a grafana’s data source.

Adding Loki as grafana data source will ensure that we can visualize the logs we have collected using fluentbit and query the logs using LogQL.

1. Let’s setup loki server running locally as container. Add `loki` service to the `docker-compose.yml` file:

    ```
    services:
      # ... services definition
      loki:
        image:grafana/loki:2.9.2
        ports:
          -"3100:3100"
        volumes:
          -./scripts/loki:/etc/loki
        command:-config.file=/etc/loki/config.yaml
    ```

   This will mount `scripts/loki` directory to the container and use `config.yaml` we will create later.

2. Add new `config.yaml` file under `scripts/loki` directory with the following content:

    ```
    auth_enabled:false
    
    server:
      http_listen_port:3100
      grpc_server_max_recv_msg_size:20971520
    
    limits_config:
      ingestion_rate_mb:10
      ingestion_burst_size_mb:20
      per_stream_rate_limit:10MB
      per_stream_rate_limit_burst:20MB
    
    common:
      path_prefix:/loki
      storage:
        filesystem:
          chunks_directory:/loki/chunks
          rules_directory:/loki/rules
      replication_factor:1
      ring:
        kvstore:
          store:inmemory
    
    table_manager:
      retention_deletes_enabled:true
      retention_period:24h
    
    schema_config:
      configs:
        -from:2020-10-24
          store:boltdb-shipper
          object_store:filesystem
          schema:v11
          index:
            prefix:index_
            period:24h
    ```

   The configuration above define few basic configurations about data retention, storage, ingestion limit, etc. This is not default configuration you might see in its documentation. I personally have been using this configuration to perform some load testing for my local development and it works well for me this far.

   If you want to learn more about the configuration, please read the **configuration documentation**.

3. You can now run `docker compose up -d` to start the `loki` container. To see if the container is running, please check the container logs.

    ```
    docker compose logs -f loki
    ```

   Please make sure there is no error in the logs.


At this point, we have loki running locally. Now, we are going to add loki as a datasource to grafana.

1. Update `scripts/grafana/provisioning/datasources/datasource.yml` file with the following content:

    ```
    apiVersion:1
    datasources:
    -name:Loki
      type:loki
      access:proxy
      orgId:1
      url:http://loki:3100
      basicAuth:false
      isDefault:false
      version:1
      editable:false
    ```

   This configuration tells grafana to use loki as its datasource. The `url` field tells grafana to use `http://loki:3100` as the loki server.

2. Restart `grafana` and `loki` container:

    ```
    docker compose restart grafana
    ```

3. Access grafana explorer at `http://localhost:3000/explore`. In datasource dropdown, you should see the `Loki` datasource is available now. But no data is available yet.

!Grafana Loki.

If you see the datasource, it means that you have successfully configured loki as grafana datasource. But, we can’t do much with this yet. In the next module, we are going to learn how to send logs to loki from data collected by updating the `fluent-bit.conf` file for fluentbit container.

## **Sending logs to Loki**

As you have already known, logs collected by fluentbit can be sent to various output plugins. In previous module, you have seen how fluentbit collects logs from a file and container’s stdout and send it to its own `stdout` output. In this section, we will send the collected logs to loki.

For more information about Loki output plugin, please read the **official documentation**.

1. To send logs collected to loki, let’s add Loki output plugin configuration to `fluent-bit.conf` stored under `scripts/fluentbit` directory.

    ```
    [OUTPUT]
        name        loki
        match       http-service
        host        loki
        port        3100
        labels      app=http-service
        drop_single_key true
        line_format key_value
    ```

   Few explanation about the config above:

    - This `match` configuration tells fluentbit to send logs that matches `http-service` tag to loki server running on `loki:3100`. Remember that earlier in **collecting logs to file module** we add `http-service` as the tag for tail input plugin.
    - The `labels` option tells fluentbit to add `app=http-service` label to all logs sent to loki. This label will be used later to filter logs in loki.
    - The `drop_single_key` option tells fluentbit to drop the key if the log only contains one key-value pair after it removes any available value configured in `lables`.
    - The `line_format` option tells fluentbit to send logs in key-value format. Since we use `drop_single_key` option, when this is configured to `key_value`, fluentbit will only sent the value of the log to loki.

   > *Later, you may try to remove `drop_single_key` and `line_format` from the config and observe what happened to the logs sent to loki.*
>
2. Restart fluentbit container

    ```
    docker compose restart fluentbit
    ```

3. Run the API server and hit API endpoint so that fluentbit can collect and send the logs to loki

    ```
    go run main.go
    curl http://localhost:8080
    ```

4. Access grafana Explore page by accessing `http://localhost:3000/explore` from your browser. Run this LogQL query:

    ```
    {app="http-service"} | json | line_format "{{.message}}"
    ```

   You should see the logs from the http service similar to this.


!http service logs in loki

That’s easy, right? Loki is only one of the many output plugins available in fluentbit. You can send logs to other output plugins such as Elasticsearch, Kafka, etc. For more information about the output plugins, please read the **official documentation**.

In the next module, we are going to learn how to query logs in loki using LogQL.

## **LogQL**

LogQL is a powerful query language designed for querying logs in Loki. Unfortunately, this won’t be a complete guide to LogQL, but rather a quick reference to the most common queries that might be useful later for you to complete the course challenge. For a complete guide, I **really** recommend you to read at least these two documentation pages about LogQL:

- **Log Queries**
- **Log Metrics**

Here are some basic LogQL queries that you should be familiar with. Add more logs to your application so that you have more data! Try this queries from your grafana explore page and make some modification as you’d like.

1. Basic text search to query logs containing an `info` keyword for logs with `app=http-service` label:

    ```
    {app="http-service"} | json |~ "info"
    ```


!logQL filter by keyword

1. Basic text retrieve logs based on specific labels (e.g. `level`) from json/structured logs:

    ```
    {app="http-service"} | json | level = "info"
    ```

2. Aggregation by counting occurrences of a specific log entry over time:

    ```
    count_over_time({app="http-service"} |~ "debug" [1m])
    ```


!LogQL count over time

The `[1m]` at the end of the query is the time range for the aggregation. It tells Loki to aggregate the log entries over the last 1 minute.

1. Calculate the rate of log entries per second in the specified interval:

    ```
    rate({app="http-service"}[1m])
    ```

2. Grouping and aggregation by counting occurrences of a log entry grouped by a specific label:

    ```
    sum by (level) (rate({app="http-service"} | json [1m]))
    ```


!LogQL sum by label

`rate` function is used to calculate the rate of log entries per second. Once we have the rate, we can run aggregation functions such as `sum`, `avg`, `min`, `max`, etc. to perform aggregation on the rate and group it by a specific label.

1. Get the top N log entries based on a specific condition:

    ```
    topk(1, sum by (level) (rate({app="http-service"} | json [1m])))
    ```

2. Another interesting query is unwrapped range aggregation. This query will unwrap the value of a specific field in the log so that you can perform aggregation on it. For example, if you have a log with the following structure:

    ```
    {
      "level":"info",
      "message":"finished call",
      "grpc_method":"GetCourse",
      "grpc_service":"course.CourseService",
      "grpc_code":"OK",
      "grpc_time_ms":0.01
    }
    ```

   To calculate the average `grpc_time_ms` the request for each grpc endpoint and status code, you can use query similar to this to do so:

    ```
    avg by (grpc_method, grpc_service, grpc_code)
      (avg_over_time({app="course-service"}
        | json
        | message = "finished call"
        | unwrap grpc_time_ms [1m]))
    ```

   `unwrap` expression is used to ensure that it uses the value from the extracted labels to perform aggregation. You can use function such as `sum_over_time`, `avg_over_time`, `min_over_time`, `max_over_time`, etc. to perform aggregation on the unwrapped value. Then, you can also use `avg by` to group the aggregation by specific labels.


There are a lot of other queries that you can do with LogQL. On the next section, there will be final challenge for you to try to create log and metrics dashboard only with logs from your http service. Let’s go!

## **Nginx log metrics exercise**

You must have an nginx container by now. The exercise is easy.

**Create a visualization in grafana dashboard to show the number of requests per seconds made to your nginx service for every different user agents.**

Hints:

- You may need to send the logs from nginx container to Loki first.
- You might need to parse nginx logs to extract the information you need.

# Challenge

1. Your users are complaining about slow response time from the application. Since you don’t have visibility about the latency, you can’t really tell whether the claim is true or not. Your task is to create a dashboard that shows the 95 percentile latency of the service only by using logs metrics. You should not use other type of instrumentation such as tracing or metrics to solve this problem. This challenges might require you to have api latency recorded on the logs.
2. our manager wants to know how many todo are made for every 5 minutes window. Since you are the only engineers, you need to add visualizations that shows the following business metrics only by using log metrics
3. **Database logs:** Stream the logs from postgres container to loki. The logs should have tag app=postgres attached when it is written to loki.
4. Database logs: On the same dashboard as other task, create a visualization that shows total number of failed login attempt for each given user name from `postgresql` container.