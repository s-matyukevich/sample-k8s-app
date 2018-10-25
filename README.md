## sample-k8s-app

### Running localy

1. Build the application image
    ```
    IMAGE=<your-dockerregistry-username>/sample-k8s-app 
    docker build . -t $IMAGE 
    ```

1. Start the DB
    ```
    docker run --rm --name db -e MYSQL_ROOT_PASSWORD=root -d mysql
    ```

1. Run the backend
    ```
    docker run --rm --name backend --link db:mysql -p 8081:8081 -d $IMAGE app -port=8081 -db-host=db -db-password=root
    ```

1. Run the frontend
    ```
    docker run --rm --name frontend --link backend -p 8080:8080 -d $IMAGE app  -frontend=true -backend-service=http://backend:8081
    ```

1. Verify that db, backend and frontend containers are running
    ```
    docker ps
    CONTAINER ID        IMAGE                         COMMAND                  CREATED              STATUS              PORTS                    NAMES
    6ed930149bff        smatyukevich/sample-k8s-app   "app -frontend=true …"   46 seconds ago       Up 43 seconds       0.0.0.0:8080->8080/tcp   frontend
    4903f99a4727        smatyukevich/sample-k8s-app   "app -port=8081 -db-…"   About a minute ago   Up About a minute   0.0.0.0:8081->8081/tcp   backend
    ca1f75c19239        mysql                         "docker-entrypoint.s…"   3 minutes ago        Up 3 minutes        3306/tcp                 db
    ```

1. Open in your browser 'http://localhost:8080' and check that the app is working

1. Cleanup
    ```
    docker stop $(docker ps -aq)
    docker rm $(docker ps -aq)
    ```
