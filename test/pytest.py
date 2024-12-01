# script.py
import time
import json
import sys
import base64
import gevent
import threading
from gevent import monkey
import select

monkey.patch_all()



def main():
    # 模拟处理
    def testgevent():
        i=2
        filename="data%d.json" % (i)
        while True:
            with open(filename, 'a', encoding='utf-8') as f:
            # 在末尾追加 JSON 数据（每次追加一个 JSON 对象）
                f.write('inside gevent' + "\n")
            rlist, _, _ = select.select([sys.stdin], [], [], 0.1)  # 0.1秒超时
            if rlist:
                input_data = sys.stdin.readline().strip()  # 获取输入并去掉换行
                print(f"输入内容: {input_data}")
            else:
                print("没有输入，继续执行其他任务...")
                gevent.sleep(1)  # 定期让出控制权

            
    #t=threading.Thread(target=testgevent)
    #t.start()
    g=gevent.spawn(testgevent)
    #g.start()
    #g.join()
    while True:
        
        result = {"a": 5, "b": "567"}
        print(json.dumps(result),flush=True)  # 输出 JSON 数据
        i=1
        filename="data%d.json" % (i)
        with open(filename, 'a', encoding='utf-8') as f:
        # 在末尾追加 JSON 数据（每次追加一个 JSON 对象）
            f.write(json.dumps(result, indent=4) + "\n")
        time.sleep(1)

if __name__ == "__main__":
    main()
