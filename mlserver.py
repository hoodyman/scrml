from concurrent import futures
import grpc
import logging
import mlserver_pb2
import mlserver_pb2_grpc
from  keras import models
from keras import layers
from keras import datasets
from keras import utils
from keras import callbacks
from datetime import datetime
import os.path
import numpy as np
import os
# os.environ['CUDA_VISIBLE_DEVICES'] = '-1'


# elogf = open('mlservererror.log', 'w')
# sys.stderr = elogf

# def lprint(*args, **kwargs):
#     with open("mlserverout.log", 'w') as logf:
#         print(*args, file=logf, **kwargs)
#     print(*args, **kwargs)


weights_file_name = "mlserver_weights"

def byteToFloat(x):
    return x / 255

class Messager(mlserver_pb2_grpc.MessagerServicer):
    learnXBuf2 = None
    samplesXBuffer = None
    samplesYBuffer = None
    model = None
    num_classes = 2
    startPopSmpl = False
    popSamplT = None
    layers = 3

    def fdp(self, d):
        stride = 3
        i = 0
        sq = []
        for y in range(self.input_shape[1]):
            row = []
            for x in range(self.input_shape[0]):
                # point = (0.3 * byteToFloat(d[i + 0])) + (0.59 * byteToFloat(d[i + 1])) + (0.11 * byteToFloat(d[i + 2]))
                point = [byteToFloat(d[i + 0]), byteToFloat(d[i + 1]), byteToFloat(d[i + 2])]
                row.append(point)
                i += stride
            sq.append(row)
        return sq

    def AppendTrainingSample(self, request, context):
        d = request.XData
        sq = self.fdp(d)

        self.learnXBuf2.append(sq)

        if len(self.learnXBuf2) == 1000:
            if self.samplesXBuffer is None:
                self.samplesXBuffer = np.array(self.learnXBuf2)
            else:
                self.samplesXBuffer = np.append(self.samplesXBuffer, np.array(self.learnXBuf2), axis=0)
            self.learnXBuf2 = []

        if self.samplesYBuffer is None:
            self.samplesYBuffer = np.array([request.YData])
        else:
            self.samplesYBuffer = np.append(self.samplesYBuffer, np.array([request.YData]), axis=0)

        return mlserver_pb2.MsgError(Err=mlserver_pb2.ENUMERROR_NOERROR)

    def AppendPredictSample(self, request, context):
        if self.startPopSmpl == False:
            self.startPopSmpl = True
            self.popSamplT = datetime.now()
        d = request.Data
        sq = self.fdp(d)

        if self.samplesXBuffer is None:
            self.samplesXBuffer = np.array([sq])
        else:
            self.samplesXBuffer = np.append(self.samplesXBuffer, np.array([sq]), axis=0)

        return mlserver_pb2.MsgError(Err=mlserver_pb2.ENUMERROR_NOERROR)

    def Predict(self, request, context):
        if self.startPopSmpl == True:
            self.startPopSmpl = False
            print("Samples load time:", datetime.now() - self.popSamplT)

        if self.model == None:
            # return mlserver_pb2.MsgPredOut(Data=b'', Err=mlserver_pb2.ENUMERROR_NOOUTDATA)
            self.model = self.BuildModel()

        # print(self.samplesXBuffer)

        t0 = datetime.now()
        y = self.model.predict(self.samplesXBuffer)
        print("Predict time:", datetime.now() - t0)

        y = np.split(y, 2, 1)
        y = y[1].reshape(1, len(y[1]))[0]
        y = np.ceil(y * 255)

        b = y.astype('uint8')
        b2 = b.tobytes()

        # print(b)

        return mlserver_pb2.MsgPredOut(Data=b2, Err=mlserver_pb2.ENUMERROR_NOERROR)

    def Test(self, request, context):
        return mlserver_pb2.VoidMsg()

    def InitMlParams(self, request, context):
        self.samplesXBuffer = None
        self.samplesYBuffer = None
        self.learnXBuf2 = []

        ss = request.SampleSize
        self.input_shape = (ss, ss, self.layers)

        return mlserver_pb2.VoidMsg()

    def Train(self, request, context):

        if len(self.learnXBuf2) > 0:
            if self.samplesXBuffer is None:
                self.samplesXBuffer = np.array([self.learnXBuf2])
            else:
                self.samplesXBuffer = np.append(self.samplesXBuffer, np.array(self.learnXBuf2), axis=0)
            self.learnXBuf2 = []

        if self.samplesXBuffer.shape[0] == 1:
            self.samplesXBuffer = self.samplesXBuffer[0]

        print(self.samplesXBuffer.shape)

        x_train = self.samplesXBuffer
        y_train = self.samplesYBuffer

        x_train = np.expand_dims(x_train, -1)
        y_train = utils.to_categorical(y_train, self.num_classes)

        self.model = self.BuildModel(l=False)

        callback = callbacks.EarlyStopping(monitor='loss', patience=5, min_delta=0.01)
        self.model.fit(x_train, y_train, batch_size=100, epochs=100, validation_split=0.1, callbacks=[callback])

        print("Save weights")
        w = self.model.get_weights()
        np.save(weights_file_name, w)
        print("OK")

        self.samplesXBuffer = np.array([])
        self.samplesYBuffer = np.array([])

        return mlserver_pb2.VoidMsg()

    def BuildModel(self, l = True):
        model = models.Sequential()

        #
        model.add(layers.Input(shape=self.input_shape)) # 64x64x3
        model.add(layers.Conv2D(32, kernel_size=(3, 3), padding="same", activation="relu"))
        model.add(layers.Conv2D(32, kernel_size=(3, 3), padding="same", activation="relu"))
        model.add(layers.Conv2D(16, kernel_size=(3, 3), padding="same", activation="relu"))
        model.add(layers.Conv2D(16, kernel_size=(3, 3), padding="same", activation="relu"))
        model.add(layers.MaxPool2D()) # 32x32x3
        model.add(layers.Conv2D(64, kernel_size=(3, 3), padding="same", activation="relu"))
        model.add(layers.Conv2D(64, kernel_size=(3, 3), padding="same", activation="relu"))
        model.add(layers.Conv2D(64, kernel_size=(3, 3), padding="same", activation="relu"))
        model.add(layers.MaxPool2D()) # 16x16x3
        model.add(layers.Conv2D(128, kernel_size=(3, 3), padding="same", activation="relu"))
        model.add(layers.MaxPool2D()) # 8x8x3
        model.add(layers.Conv2D(256, kernel_size=(3, 3), padding="same", activation="relu"))
        model.add(layers.MaxPool2D()) # 4x4x3
        model.add(layers.Conv2D(512, kernel_size=(3, 3), padding="same", activation="relu"))

        model.add(layers.Flatten())
        model.add(layers.Dropout(0.5))
        model.add(layers.Dense(self.num_classes, activation="softmax"))

        model.summary()

        model.compile(loss="categorical_crossentropy", optimizer="adam", metrics=["accuracy"])

        if l == True:
            we = os.path.exists(weights_file_name+'.npy')
            if we:
                w = np.load(file=weights_file_name+'.npy', allow_pickle=True)
                try:
                    model.set_weights(w)
                except ValueError:
                    pass # ignore it

        return model

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=1))
    mlserver_pb2_grpc.add_MessagerServicer_to_server(Messager(), server)
    server.add_insecure_port('[::]:50555')
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    logging.basicConfig()
    print("READY")
    serve()
