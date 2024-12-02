
import gevent
from gevent import monkey
monkey.patch_all()
import random
import sys
import time
import pickle
import base64
import json
import select


from gevent.queue import Queue

from honeybadgerbft.core.honeybadger import HoneyBadgerBFT
#from honeybadgerbft.crypto.threshsig.boldyreva import dealer
#from honeybadgerbft.crypto.threshenc import tpke

    
def run_honeybadger(N,f,i,r,input):
    filename="/home/ubuntu/golang/src/dumbo_ms/log/data%d.json" % (i)
    with open(filename, 'a', encoding='utf-8') as file:
        m="inside honeybadger BFT consensus %d %d %d %d \n" % (N,f,i,r)
        file.write(m)
    #recv_queue=Queue()
    
    def send(j,o):
        serialized_o=pickle.dumps(o)
        encoded_data = base64.b64encode(serialized_o).decode('utf-8')
        cons_msg_out={"sendid":i,"revid":j,"priority":r,"content":encoded_data,"type":1}
        print(json.dumps(cons_msg_out),flush=True)
        #filename="/home/luy/golang_projects/src/dumbo_ms/log/data%d.json" % (i)
        #with open(filename, 'a', encoding='utf-8') as file:
        #   file.write('send a message')
        #   file.write(json.dumps(cons_msg_out, indent=4) + "\n")
    
    def recv():
        while True:
            rlist, _, _ = select.select([sys.stdin], [], [], 0.1)  # 0.1秒超时
            if rlist:
                input_data = sys.stdin.readline().strip()  # 获取输入并去掉换行
                #line=sys.stdin.readline().strip()
                payload=json.loads(input_data)
                sender=payload['sendid']
                content=payload['content']
                decoded_data = base64.b64decode(content)
                #filename="data%d.json" % (i)
                #filename="/home/luy/golang_projects/src/dumbo_ms/log/data%d.json" % (i)
                #with open(filename, 'a', encoding='utf-8') as file:
                #    file.write("got a message in receive" + "\n")
                #    file.write(json.dumps(payload, indent=4) + "\n")
                o=pickle.loads(decoded_data)
                #(i,o) = recv_queue.get()
                return (sender,o)
            else:
                gevent.sleep(0.0001)
    
    hb=HoneyBadgerBFT('sid',i,N,f,send,recv,r)
    hb.submit_tx(input)
            
    #gevent.spawn(recv_net)
    with open(filename, 'a', encoding='utf-8') as file:
    # 在末尾追加 JSON 数据（每次追加一个 JSON 对象）
        file.write('start running honeybadger BFT consensus' + "\n")
    
    out = hb.run()
    serialized_ot=pickle.dumps(out)
    encoded_data = base64.b64encode(serialized_ot).decode('utf-8')
    cons_msg_out={"sendid":i,"revid":-1,"priority":r,"content":encoded_data,"type":1}
    print(json.dumps(cons_msg_out),flush=True)

    with open(filename, 'a', encoding='utf-8') as file:
    # 在末尾追加 JSON 数据（每次追加一个 JSON 对象）
        file.write('get output of honeybadger BFT consensus' + "\n")
    
    return

if __name__ == '__main__':
    n = int(sys.argv[1])
    byz = int(sys.argv[2])
    id = int(sys.argv[3])
    r = int(sys.argv[4])
    input= sys.argv[5]
    run_honeybadger(n,byz,id,r,input)
