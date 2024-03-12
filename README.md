# my-bluearcus-project

Overview
Welcome to my Full Stack demonstration project! This project showcases my proficiency in developing both front-end and back-end applications using Vue.js for the front end, and Go (Golang) and PostgreSQL for the back end.

Features
Real-time Chart Updates: Utilizes WebSocket to provide live updates to the charts displayed on the front end.
Multiple Chart Types: Supports drawing two different types of charts on the front end.
Database Schema: The PostgreSQL database contains columns for x, y, date, and graph_type. It assumes that for a given date, a single data point is present in the database.
RESTful API Endpoint: Includes an endpoint to add data to the database, with example cURL command provided within the code.
Data Aggregation: When multiple x and y pairs are present within a given time range, the backend sums up all y values corresponding to a particular x value before displaying them on the front end.
