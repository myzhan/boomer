# coding: utf8

from locust import Locust, TaskSet, task

class MyTaskSet(TaskSet):
    @task(20)
    def hello(self):
        pass

class Dummy(Locust):
    task_set = MyTaskSet
