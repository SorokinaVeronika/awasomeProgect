The server is responsible for fetching data about Exchange-Traded Funds (ETFs) from the website https://www.ssga.com. These ETF data are updated on a daily basis, ensuring that the information is always current.

To facilitate API testing, you can find a Postman collection in the project's root directory.

Before starting the server, make sure you have PostgreSQL installed. To run the database, execute the following command in your terminal while in the project's root directory:

docker-compose up -d

This command will launch a PostgreSQL container.
