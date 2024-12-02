#from ..crypto.threshenc import tpke
import os



def honeybadger_block_without_encryption(pid,N, f,propose_in, acs_in, acs_out):
    """The HoneyBadgerBFT algorithm for a single block without threshold encryption

    :param pid: my identifier
    :param N: number of nodes
    :param f: fault tolerance
    :param PK: threshold encryption public key
    :param SK: threshold encryption secret key
    :param propose_in: a function returning a sequence of transactions
    :param acs_in: a function to provide input to acs routine
    :param acs_out: a blocking function that returns an array of ciphertexts
    :param tpke_bcast:
    :param tpke_recv:
    :return:
    """

    # Broadcast inputs are of the form (tenc(key), enc(key, transactions))

    # Threshold encrypt
    # TODO: check that propose_in is the correct length, not too large
    
    #filename="/home/luy/golang_projects/src/dumbo_ms/log/data%d.json" % (pid)
    #with open(filename, 'a', encoding='utf-8') as file:
    #    file.write('inside consensus honeybadger_block_without_encryption' + "\n")
            
    prop = propose_in()
    acs_in(prop)
    #with open(filename, 'a', encoding='utf-8') as file:
    #    file.write('inside consensus generate input to acs' + "\n")
    # Wait for the corresponding ACS to finish
    vall = acs_out()
    assert len(vall) == N
    assert len([_ for _ in vall if _ is not None]) >= N - f  # This many must succeed

    # print pid, 'Received from acs:', vall

    # Broadcast all our decryption shares
    my_shares = []
    for _, v in enumerate(vall):
        if v is None:
            my_shares.append(None)
            continue
        my_shares.append(v)


    return tuple(my_shares)