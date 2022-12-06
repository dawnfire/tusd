<script>

import * as tus from 'tus-js-client';
import forge from 'node-forge';


export default {
  props: {
    serveURL: { type: String, default: '/api/file' },
  },
  data() {
    return {
      criteria: '.*',
      dragover: false,
      dragleave: false,
      dropped: false,

      title: '文件服务测试场',
      transmitted: [],
      progress: 0.0,
      performance: 0.0,
    }
  },
  mounted() {
    setTimeout(() => {
      this.queryFiles();
    }, 128);
  },
  methods: {
    removeFile(fileID, fileName) {
      fetch(`${this.serveURL}/${fileID}`, {
        method: 'DELETE',
        headers: {
          'Tus-Resumable': '1.0.0'
        }
      }).then(v => {
        console.log(v.status);
        if (v.status !== 204) {
          console.log('delete failed');
          return;
        }
        console.log(`成功删除${fileName}`);
        let idx = this.transmitted.findIndex(e => e.full.ID === fileID);
        if (idx < 0) {
          console.log(`find ${fileName} failed`);
          return;
        }
        this.transmitted.splice(idx, 1);
      }).catch(err => console.log(err));
    },
    queryFiles() {
      let v = encodeURIComponent(this.criteria);
      fetch(this.serveURL + `/nonexistence?q=${v}`).then(v => {
        let size = v.headers.get('content-length');
        if (!v || size === '0') {
          return [];
        }

        return v.json();
      }).then(v => {
        if (!v || v.length == 0) {
          console.log('empty file list');
          return;
        }

        let d = [];
        for (let i = 0; i < v.length; i++) {
          let metadata = v[i].MetaData;
          metadata.url = `${this.serveURL}/${v[i].ID}`;
          metadata.full = v[i];
          if (!metadata.filename) {
            metadata.filename = v[i].ID;
          }

          if (!metadata.filesize) {
            metadata.filesize = v[i].Size;
          }

          if (!metadata.checksum) {
            metadata.checksum = v[i].ID;
          }

          d.push(metadata);
        }
        this.transmitted = d;
      }).catch(err => {
        console.log(err);
      });
    },
    digest(file) {
      return new Promise((resolve, reject) => {
        let md = forge.md.sha256.create();
        const CHUNK_SIZE = 4 * 1024 * 1024;

        let fileReader = new FileReader();

        let read = 0;
        fileReader.onload = (e) => {
          if (!e || !e.target || !e.target.result) {
            let err = new Error('invalid event.target.result');
            console.log(err.message);
            return;
          }

          read += e.target.result.length;
          md.update(e.target.result);
          seek();
        };

        let fileSize = file.size;
        let start = 0, end = 0;

        let seek = () => {
          let now = new Date();
          this.performance = (read * 1.0 / (now.getTime() - beginTime.getTime())) * 1000 / (1024 * 1024);

          this.progress = ((read * 1.0) / fileSize * 100).toFixed(2);
          if (read >= fileSize) {
            console.log(`read completed: ${read}, fileSize: ${fileSize}`);
            resolve(md.digest().toHex());
            return;
          }

          end += CHUNK_SIZE;
          end = end < fileSize ? end : fileSize + 1;
          let slice = file.slice(start, end);

          fileReader.readAsBinaryString(slice);
          start = end;
        };

        let beginTime = new Date();
        seek();
      });
    },
    transport(files) {
      return new Promise((resolve, reject) => {
        if (!files || files.length == 0) {
          let errMsg = 'invalid files';
          let err = new Error(errMsg);
          console.log(err.message);
          reject(err);
          return;
        }

        let transmitted = [];
        for (let i = 0; i < files.length; i++) {
          let f = files[i];
          if (!f.name || !f.size) {
            let err = new Error(`invalid files[${i}]`);
            console.log(err.message);
            reject(err);
            return;
          }

          this.digest(f).then(checksum => {
            let metadata = {
              filename: f.name,
              filetype: f.type,
              filesize: f.size,
              lastModified: f.lastModified,
              checksum,
            };

            let upload = new tus.Upload(f, {
              endpoint: this.serveURL,
              metadata,
              onError: (err) => {
                console.log(err);
                reject(err);
              },
              onAfterResponse: (req, resp) => { },
              onSuccess: () => {
                metadata.url = upload.url;
                transmitted.push(metadata);
                if (transmitted.length !== files.length) {
                  return;
                }

                resolve(transmitted);
              },
              onProgress: (bytesUploaded, bytesTotal) => {
                console.log(`${bytesUploaded}/${bytesTotal}`)
              },
            });

            // Check if there are any previous uploads to continue.
            upload.findPreviousUploads().then((previousUploads) => {
              // Found previous uploads so we select the first one.
              if (previousUploads.length) {
                upload.resumeFromPreviousUpload(previousUploads[0])
              }

              // Start the upload
              upload.start()
            });
          }).catch(err => {
            console.log(err);
            reject(err);
          });
        }
      });
    },

    uploadFiles(e) {
      if (!e.target.files || !(e.target.files[0])) {
        console.log('no file(s) selected');
        return;
      }

      this.transport(e.target.files).then(v => {
        this.transmitted = v;
        console.log(v);
      }).catch(err => {
        console.log(err);
      });
    }
  }
}
</script>

<template>
  <main>

    <div class="header-left center">header left</div>
    <div class="header-middle title center">{{ title }}</div>
    <div class="header-right status center">local: {{ progress }}%, {{ performance.toFixed(2) }}MB</div>
    <div class="content-left  center">content left</div>
    <div class="content center">

      <div>
        <input v-model="criteria" />
        <button @click="queryFiles">查看文件</button>
      </div>
      <div :class="['dropzone', dragover ? 'dragover' : '']" ref="dropzone"
        @dragleave="() => { dragover = false; dropped = false; }"
        @drop="() => { dragover = false; }"
        @dragover="() => { dragover = true }">

        <div class="hint">
          <div>请拖放文件</div>
          <div>或点击此处</div>
        </div>
        <input class="dropzone-input-file" type="file" @change="uploadFiles" multiple ref="inputFiles" />
      </div>
      <div class="uploaded-list">
        <ul>
          <li v-for="file in transmitted" :key="file.checksum">
            <div class="file-panel">
              <div class="name"><a :href="file.url"> {{ file.filename }}</a></div>
              <div class="size">{{ file.full.Size }}</div>
              <div class="btn-wrapper resume"><button>续传</button></div>
              <div class="btn-wrapper delete"><button @click="removeFile(file.full.ID)">删除</button></div>
            </div>
          </li>
        </ul>
      </div>

    </div>
    <div class="content-right center">content right</div>
    <div class="footer center">
      <div class="action-message center">this is message area</div>

    </div>

  </main>
</template>

<style lang="scss" scoped>
main {
  display: grid;
  grid-template-rows: 3em auto 3em;
  grid-template-columns: auto auto auto;

  height: calc(100vh - 1.8em);
  width: calc(100vw - 1.8em);


  .header-left {
    grid-row: 1 / 1;
    grid-column: 1 / 1;
    visibility: hidden;
  }

  .header-right {
    grid-row: 1 / 1;
    grid-column: 3 / 3;

    &.status {
      color: cadetblue;
      font-family: "Courier New", Courier, monospace;
      font-size: 14px;
    }
  }

  .header-middle {
    grid-row: 1 / 1;
    grid-column: 2 / 2;

    &.title {
      grid-row: 1 / 1;
      grid-column: 2 / 2;
      font-size: 16px;
      font-weight: bold;
      color: indigo;
    }
  }

  .content-left {
    grid-row: 2 / 2;
    grid-column: 1 / 1;
    visibility: hidden;
  }

  .content {
    grid-row: 2 / 2;
    grid-column: 2 / 2;

    display: flex;
    flex-direction: column;

    min-height: 38.2vh;
    min-width: 38.2vw;
    margin: 0 auto;

    box-shadow: 0px 2.292px 3.225px -1.458px rgba(0, 0, 0, 0.2), 0px 5px 7.915px 0.625px rgba(0, 0, 0, 0.14), 0px 1.875px 9.580px 1.666px rgba(0, 0, 0, 0.12);

    .uploaded-list {
      ul {
        list-style: none;
      }

      .file-panel {
        display: grid;
        grid-template-columns: 4fr 4fr 1fr 1fr;

        border-bottom: 1px dotted #a8a8a8;
        padding: 0.2em;

        .name {
          display: flex;
          align-items: center;
        }

        .size {
          display: flex;
          justify-content: end;
          align-items: center;
          padding-right: 1em;
        }

        .btn-wrapper {
          padding: 0 3px;

          button {
            border-radius: 0;
            border-style: none;
            color: indigo;
            background-color: white;

            &:hover {
              color: white;
              background-color: rgb(16, 42, 82);
            }
          }
        }
      }
    }

    .dropzone {
      position: relative;
      border: 3px dotted lightgray;
      border-radius: 5px;

      width: 96px;
      height: 82px;

      &.dragover {
        border: 3px solid lightgray;
      }

      .hint {
        position: absolute;
        top: 0;
        right: 0;
        bottom: 0;
        left: 0;

        color: indigo;

        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        font-size: 16px;
        font-weight: bold;
      }

      .dropzone-input-file {
        position: absolute;
        opacity: 0;
        top: 0;
        right: 0;
        bottom: 0;
        left: 0;
        // border: 1px solid red;
        cursor: pointer;

        // height: 100%;
        width: 98%;
      }
    }
  }

  .content-right {
    grid-row: 2 / 2;
    grid-column: 3 / 3;
    visibility: hidden;
  }

  .footer {
    grid-row: 3 / 3;
    grid-column: 1 / span 3;

    .action-message {
      color: grey;
    }
  }

  .center {
    display: flex;
    justify-content: center;
    align-items: center;
  }
}
</style>